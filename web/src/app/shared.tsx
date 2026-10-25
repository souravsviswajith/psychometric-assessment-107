/**
 * app/shared.tsx — data helpers for the app screens.
 *
 * Everything here is derived from the Go runtime's state. The "% matched"
 * figure is the share of a direction's requirements that the user's answers
 * support; it is computed for display only and never stored in the domain.
 */
import type {
  CaptureInput,
  InstrumentItem,
  PathwayCandidate,
  PathwayRequirement,
  RequirementStatus,
  ReviewState,
  Signal,
  SessionView,
} from '../api/types.ts';

// ----------------------------------------------------------------- profile

export interface Profile {
  readonly name: string;
  readonly setting: string;
  readonly careerStage: string;
}

export const SETTING_OPTIONS: readonly { value: string; label: string }[] = [
  { value: 'studying', label: 'Studying' },
  { value: 'employed full-time', label: 'Working full-time' },
  { value: 'self-employed', label: 'Self-employed' },
  { value: 'between roles', label: 'Between jobs' },
  { value: 'not working', label: 'Not working' },
];

export const STAGE_OPTIONS: readonly { value: string; label: string }[] = [
  { value: 'early-career', label: 'Starting out' },
  { value: 'mid-career', label: 'A few years in' },
  { value: 'late-career', label: 'Experienced' },
  { value: 'returning after a break', label: 'Returning' },
];

export function labelFor(options: readonly { value: string; label: string }[], value: string): string {
  return options.find((option) => option.value === value)?.label ?? value;
}

const PROFILE_KEY = 'career107.profile';

/** The profile is a per-device convenience; answers carry their own context. */
export function loadProfile(): Profile | undefined {
  try {
    const parsed = JSON.parse(window.localStorage.getItem(PROFILE_KEY) ?? 'null') as Partial<Profile> | null;
    if (typeof parsed?.setting === 'string' && typeof parsed.careerStage === 'string') {
      return { name: typeof parsed.name === 'string' ? parsed.name : '', setting: parsed.setting, careerStage: parsed.careerStage };
    }
  } catch {
    // storage unavailable
  }
  return undefined;
}

export function saveProfile(profile: Profile | undefined): void {
  try {
    if (profile === undefined) window.localStorage.removeItem(PROFILE_KEY);
    else window.localStorage.setItem(PROFILE_KEY, JSON.stringify(profile));
  } catch {
    // storage unavailable: the profile lives for this visit only
  }
}

export function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  return (parts.length === 0 ? 'ME' : parts.length === 1 ? parts[0]!.slice(0, 2) : parts[0]![0]! + parts.at(-1)![0]!).toUpperCase();
}

/** A stable candidate reference derived from the first recorded observation. */
export function candidateId(view: SessionView): string {
  const first = view.signals[0];
  return first === undefined ? 'Pending' : `#CA-${first.id.slice(4, 8).toUpperCase()}-${first.id.slice(8, 9).toUpperCase()}`;
}

// ----------------------------------------------------------------- answers

export function buildAnswer(item: InstrumentItem, response: CaptureInput['response'], profile: Profile): CaptureInput {
  return {
    instrumentId: item.instrumentId,
    itemRef: item.itemRef,
    response,
    declaredContext: [
      { key: 'setting', value: profile.setting },
      { key: 'careerStage', value: profile.careerStage },
      { key: 'sessionMode', value: 'self-directed (app)' },
    ],
    source: { kind: 'instrument-administration', ref: `app://session/${item.itemRef}`, label: 'app sitting' },
    sessionRef: 'session.app-1',
  };
}

/** The latest answer per question (a changed answer is a new observation). */
export function latestAnswers(view: SessionView): Map<string, Signal> {
  return new Map(view.signals.map((signal) => [signal.instrument.itemRef, signal]));
}

export function answerText(signal: Signal): string {
  const r = signal.response;
  if (r.kind === 'ordinal') return r.presentedLabels?.[r.rawIndex ?? 0] ?? '—';
  if (r.kind === 'categorical') return (r.selectedLabels ?? []).join(', ');
  return r.text ?? '';
}

export function selectedOptionId(signal: Signal | undefined, item: InstrumentItem): string | undefined {
  const r = signal?.response;
  if (r?.kind === 'categorical') return r.selectedOptionIds?.[0];
  if (r?.kind === 'ordinal') return item.options[r.rawIndex ?? -1]?.id;
  return undefined;
}

/** "Describes me fully (position 1.00)" → "Describes me fully". */
export function plainObservation(value: string): string {
  return value.replace(/ \(position [\d.]+\)$/, '');
}

export function formatDate(iso: string): string {
  const date = new Date(iso);
  return Number.isNaN(date.getTime()) ? iso : date.toLocaleString(undefined, { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });
}

// ----------------------------------------------------------------- modules

/** The battery is presented in modules, by the kind of construct each item reads. */
export const MODULES: readonly { title: string; kinds: readonly string[] }[] = [
  { title: 'Interests & Affinity', kinds: ['interest'] },
  { title: 'Values & Motivation', kinds: ['value', 'motivation'] },
  { title: 'Working Style', kinds: ['work-style', 'preference'] },
  { title: 'Skills & Experience', kinds: ['skill-self-report', 'experience'] },
  { title: 'Context & Constraints', kinds: ['constraint', 'context-condition'] },
];

export function moduleIndex(item: InstrumentItem): number {
  const index = MODULES.findIndex((module) => module.kinds.includes(item.constructs[0]?.kind ?? ''));
  return index < 0 ? MODULES.length - 1 : index;
}

/** Items in battery order: grouped by module, original order within a module. */
export function batteryOrder(items: readonly InstrumentItem[]): InstrumentItem[] {
  return [...items].sort((a, b) => moduleIndex(a) - moduleIndex(b));
}

/** Short labels for the 1–5 response matrix. */
export function shortLabel(label: string): string {
  return (
    {
      'Does not describe me': 'Not me',
      'Describes me slightly': 'Slightly',
      'Describes me partly': 'Partly',
      'Describes me mostly': 'Mostly',
      'Describes me fully': 'Fully',
      Consistently: 'Always',
      Sometimes: 'Some',
    }[label] ?? label
  );
}

// ---------------------------------------------------------------- pathways

export function matchPercent(pathway: PathwayCandidate): number {
  const { supported, total } = pathway.supportSummary;
  return total === 0 ? 0 : Math.round((supported / total) * 100);
}

/** Directions by share of requirements matched, then catalogue order. */
export function byMatch(pathways: readonly PathwayCandidate[]): PathwayCandidate[] {
  return [...pathways].sort((a, b) => matchPercent(b) - matchPercent(a));
}

export const FAMILY_ICON: Record<string, string> = {
  analysis: 'query_stats',
  building: 'architecture',
  operations: 'support_agent',
  coordination: 'account_tree',
  people: 'groups',
  physical: 'construction',
};

/** The user's three highest self-ratings on interest, value, style and skill items. */
export function strengths(view: SessionView, items: readonly InstrumentItem[]): { label: string; percent: number }[] {
  const answers = latestAnswers(view);
  return items
    .map((item) => ({ item, response: answers.get(item.itemRef)?.response }))
    .filter(({ item, response }) => response?.kind === 'ordinal' && moduleIndex(item) < 4)
    .map(({ item, response }) => ({ label: item.constructs[0]?.label ?? item.itemRef, percent: Math.round((response?.position ?? 0) * 100) }))
    .sort((a, b) => b.percent - a.percent)
    .slice(0, 3);
}

// ---------------------------------------------------------------- statuses

export const STATUSES: readonly RequirementStatus[] = ['supported', 'gap', 'conflicting', 'undetermined'];

export const STATUS_TEXT: Record<RequirementStatus, { short: string; long: string; icon: string }> = {
  supported: { short: 'Met', long: 'Matches your answers', icon: 'check_circle' },
  gap: { short: 'Not met', long: "Doesn't match your answers", icon: 'cancel' },
  conflicting: { short: 'Mixed', long: 'Your answers point both ways', icon: 'contrast' },
  undetermined: { short: 'Open', long: 'Not enough information yet', icon: 'pending' },
};

/**
 * The mapping's rationale cites bands and policy ids, which is right for an
 * audit but not for a person. This says the same in plain words.
 */
export function plainReason(requirement: PathwayRequirement): string {
  const { status, kind, rationale, observations } = requirement;
  if (kind === 'constraint') return rationale;
  if (status === 'undetermined') {
    if (observations.length === 0) return "None of your answers covers this yet, so it can't be read either way.";
    if (rationale.startsWith('Every observation')) return "Your answer can't be read for this yet — for example, part of your situation is missing.";
    return "Your answer was in the middle of the scale, so it doesn't point either way.";
  }
  if (status === 'conflicting') return "You answered related questions in opposite ways. That can happen when your situation is changing; the app doesn't pick one for you.";
  if (rationale.includes('only asks whether it is present')) return 'You answered this, and this direction only needs to know it has been considered.';
  const expectsLow = rationale.includes('expects a low reading');
  if (status === 'supported') {
    return expectsLow ? 'This direction suits a lower answer here, and yours was low.' : 'Your answer was at the strong end of the scale, which is what this direction looks for.';
  }
  return expectsLow ? 'This direction suits a lower answer here, but yours was high.' : 'Your answer was at the low end of the scale, while this direction usually needs more of it.';
}

/** Review states the user can choose, with the decision record kind each produces. */
export const DECISIONS: readonly { state: Exclude<ReviewState, 'unreviewed'>; kind: string; label: string; hint: string }[] = [
  { state: 'pursuing', kind: 'pursue-pathway', label: 'Pursue', hint: 'I want to go for this' },
  { state: 'interesting', kind: 'explore-pathway', label: 'Explore', hint: 'Worth looking into' },
  { state: 'deferred', kind: 'defer-pathway', label: 'Later', hint: 'Not right now' },
  { state: 'not-a-fit', kind: 'dismiss-pathway', label: 'Not for me', hint: "Doesn't suit me" },
];

/** Label for a review state or a decision record kind. */
export function decisionLabel(stateOrKind: string): string | undefined {
  return DECISIONS.find((decision) => decision.state === stateOrKind || decision.kind === stateOrKind)?.label;
}
