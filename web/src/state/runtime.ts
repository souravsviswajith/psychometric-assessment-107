/**
 * state/runtime.ts — the UI's hold on the Go runtime.
 *
 * The UI never owns career state: it asks the runtime (served by Go, in the
 * app or on a server) for domain state, and re-renders. Everything shown is
 * derived from domain objects, so the UI cannot drift into being the model
 * (§14).
 */
import { useCallback, useEffect, useState } from 'react';
import type { ActionResponse, CaptureInput, Catalog, ResearchRun, ReviewState, SessionView } from '../api/types.ts';

async function request<T>(path: string, body?: unknown): Promise<T> {
  const response = await fetch(`api/${path}`, {
    method: body === undefined ? 'GET' : 'POST',
    headers: body === undefined ? {} : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!response.ok) {
    let message = `${response.status} ${response.statusText}`;
    try {
      const payload = (await response.json()) as { error?: string };
      if (payload.error !== undefined) message = payload.error;
    } catch {
      // keep the status text
    }
    throw new Error(message);
  }
  return (await response.json()) as T;
}

export interface SessionApi {
  readonly catalog: Catalog;
  readonly view: SessionView;
  readonly notice: string | undefined;
  readonly rejections: readonly string[];
  readonly busy: boolean;
  capture(input: CaptureInput): Promise<boolean>;
  /** Capture and decompose in one step (the app's question flow). */
  answer(input: CaptureInput): Promise<boolean>;
  decompose(signalId: string): Promise<boolean>;
  mapPathways(): Promise<boolean>;
  reviewPathway(pathwayId: string, state: Exclude<ReviewState, 'unreviewed'>, rationale?: string): Promise<boolean>;
  correctRepresentation(representationId: string, correctedSummary: string): Promise<boolean>;
  proposeQuestions(representationId: string): Promise<boolean>;
  runResearch(): Promise<ResearchRun>;
  reset(): Promise<boolean>;
  clearMessages(): void;
}

export type SessionState =
  | { readonly status: 'loading' }
  | { readonly status: 'failed'; readonly error: string; retry(): void }
  | { readonly status: 'ready'; readonly api: SessionApi };

export function useCareerSession(): SessionState {
  const [catalog, setCatalog] = useState<Catalog | undefined>(undefined);
  const [view, setView] = useState<SessionView | undefined>(undefined);
  const [loadError, setLoadError] = useState<string | undefined>(undefined);
  const [attempt, setAttempt] = useState(0);
  const [notice, setNotice] = useState<string | undefined>(undefined);
  const [rejections, setRejections] = useState<readonly string[]>([]);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let cancelled = false;
    setLoadError(undefined);
    Promise.all([request<Catalog>('catalog'), request<SessionView>('session')])
      .then(([loadedCatalog, loadedView]) => {
        if (cancelled) return;
        setCatalog(loadedCatalog);
        setView(loadedView);
      })
      .catch((error: unknown) => {
        if (!cancelled) setLoadError(error instanceof Error ? error.message : String(error));
      });
    return () => {
      cancelled = true;
    };
  }, [attempt]);

  const act = useCallback(async (path: string, body: unknown): Promise<boolean> => {
    setBusy(true);
    try {
      const result = await request<ActionResponse>(path, body);
      setView(result.view);
      setRejections(result.rejections);
      setNotice(result.rejections.length > 0 ? undefined : result.notice);
      return result.rejections.length === 0;
    } catch (error) {
      setNotice(undefined);
      setRejections([`REQUEST.FAILED: ${error instanceof Error ? error.message : String(error)}`]);
      return false;
    } finally {
      setBusy(false);
    }
  }, []);

  if (loadError !== undefined) {
    return { status: 'failed', error: loadError, retry: () => setAttempt((value) => value + 1) };
  }
  if (catalog === undefined || view === undefined) return { status: 'loading' };

  return {
    status: 'ready',
    api: {
      catalog,
      view,
      notice,
      rejections,
      busy,
      capture: (input) => act('capture', input),
      answer: (input) => act('answer', input),
      decompose: (signalId) => act('decompose', { signalId }),
      mapPathways: () => act('map', {}),
      reviewPathway: (pathwayId, state, rationale) =>
        act('review', rationale === undefined ? { pathwayId, state } : { pathwayId, state, rationale }),
      correctRepresentation: (representationId, correctedSummary) => act('correct', { representationId, correctedSummary }),
      proposeQuestions: (representationId) => act('propose', { representationId }),
      runResearch: () => request<ResearchRun>('research/run', {}),
      reset: () => act('reset', {}),
      clearMessages: () => {
        setNotice(undefined);
        setRejections([]);
      },
    },
  };
}
