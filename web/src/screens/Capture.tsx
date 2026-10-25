import { useState, type ReactNode } from 'react';
import type { CaptureInput, InstrumentItem } from '../api/types.ts';
import { Badge, KV, Notice, Panel, Stat } from '../components/atoms.tsx';
import type { SessionApi } from '../state/runtime.ts';

const CONTEXT_PRESETS: readonly { key: string; label: string; options: readonly string[] }[] = [
  { key: 'setting', label: 'Setting you are assessing this from', options: ['employed full-time', 'between roles', 'studying', 'self-employed', 'not working'] },
  { key: 'careerStage', label: 'Where you are in your working life', options: ['early-career', 'mid-career', 'late-career', 'returning after a break'] },
];

export function CaptureScreen(props: { api: SessionApi }): ReactNode {
  const items = props.api.catalog.items;
  const [itemRef, setItemRef] = useState(items[0]?.itemRef ?? '');
  const [selection, setSelection] = useState<string>('');
  const [text, setText] = useState<string>('');
  const [context, setContext] = useState<{ setting: string; careerStage: string }>({
    setting: CONTEXT_PRESETS[0]!.options[0]!,
    careerStage: CONTEXT_PRESETS[1]!.options[1]!,
  });

  const item: InstrumentItem | undefined = items.find((candidate) => candidate.itemRef === itemRef);
  const openTextSignals = props.api.view.signals.filter((signal) => signal.instrument.responseShape === 'open-text');

  const submit = (): void => {
    if (item === undefined) return;
    const response: CaptureInput['response'] =
      item.responseShape === 'open-text'
        ? { kind: 'text', text: text.trim().length === 0 ? 'No answer given.' : text.trim() }
        : { kind: 'option', optionId: selection === '' ? (item.options[0]?.id ?? '') : selection };

    void props.api.capture({
      instrumentId: item.instrumentId,
      itemRef: item.itemRef,
      response,
      declaredContext: [
        { key: 'setting', value: context.setting },
        { key: 'careerStage', value: context.careerStage },
        { key: 'sessionMode', value: 'self-directed (app)' },
      ],
      source: { kind: 'instrument-administration', ref: `app://session/${item.itemRef}`, label: 'app sitting' },
      sessionRef: 'session.app-1',
    });
    setSelection('');
    setText('');
  };

  const latest = props.api.view.signals.at(-1);

  return (
    <div className="grid cols-2">
      <Panel title="Assessment input" subtitle="S0 · capture">
        <p className="panel-note">
          What you answer becomes an observation with a permanent identity. It keeps what was asked, how you answered, where the answer came
          from, when, and how uncertain the system is about it. Nothing here is scored.
        </p>

        <h3>Which question</h3>
        <select value={itemRef} onChange={(event) => setItemRef(event.target.value)}>
          {items.map((candidate) => (
            <option key={candidate.itemRef} value={candidate.itemRef}>
              {candidate.promptText}
            </option>
          ))}
        </select>

        {item === undefined ? null : (
          <>
            <Badge>{`${item.instrumentId}@${item.instrumentVersion}`}</Badge>{' '}
            <Badge>{`${item.itemRef}@${item.itemVersion}`}</Badge>{' '}
            <Badge title={item.validated ? 'Has a validation reference' : 'No validation reference recorded'}>
              {item.validated ? 'validated' : 'unvalidated instrument'}
            </Badge>

            <h3>Your answer</h3>
            {item.responseShape === 'open-text' ? (
              <textarea
                value={text}
                onChange={(event) => setText(event.target.value)}
                rows={5}
                maxLength={4000}
                placeholder="Describe it in your own words. This text is kept verbatim as evidence; interpretation is proposed separately."
                style={{
                  width: '100%',
                  borderRadius: '9px',
                  border: '1px solid var(--hairline-strong)',
                  background: 'rgba(11,16,24,0.75)',
                  color: 'var(--slate-200)',
                  padding: '9px 11px',
                  fontFamily: 'var(--sans)',
                  fontSize: '13px',
                }}
              />
            ) : (
              <div className="actions">
                {item.options.map((option) => (
                  <button
                    key={option.id}
                    type="button"
                    className={`action${selection === option.id ? ' primary' : ''}`}
                    onClick={() => setSelection(option.id)}
                  >
                    {option.label}
                  </button>
                ))}
              </div>
            )}

            <h3>Context you are answering from</h3>
            <div className="grid cols-2" style={{ gap: '8px' }}>
              {CONTEXT_PRESETS.map((preset) => (
                <label key={preset.key} className="stat">
                  <span className="label">{preset.label}</span>
                  <select
                    value={context[preset.key as 'setting' | 'careerStage']}
                    onChange={(event) => setContext({ ...context, [preset.key]: event.target.value })}
                  >
                    {preset.options.map((option) => (
                      <option key={option} value={option}>
                        {option}
                      </option>
                    ))}
                  </select>
                </label>
              ))}
            </div>

            <div className="actions">
              <button type="button" className="action primary" disabled={props.api.busy} onClick={submit}>
                Capture observation
              </button>
              {latest === undefined ? null : (
                <button type="button" className="action" onClick={() => void props.api.decompose(latest.id)}>
                  Decompose latest into (A, E, C) | U
                </button>
              )}
            </div>
          </>
        )}
      </Panel>

      <div className="stack">
        <Panel title="What the capture produced" subtitle="S0 output" tight>
          {latest === undefined ? (
            <Notice>No observation captured in this session yet.</Notice>
          ) : (
            <>
              <div className="grid cols-3">
                <Stat label="signals" value={props.api.view.signals.length} />
                <Stat label="uncertainty notes" value={latest.uncertainty.notes.length} />
                <Stat label="evidence records" value={latest.evidence.length} />
              </div>
              <h3>Identity</h3>
              <KV
                rows={[
                  ['signal.id', <span className="mono">{latest.id}</span>],
                  ['context.id', <span className="mono">{latest.context.id}</span>],
                  ['evidence.id', <span className="mono">{latest.evidence[0]?.id ?? '—'}</span>],
                  ['identity invariant', latest.context.id === latest.id && latest.evidence[0]?.id === latest.id ? <Badge tone="facet-e">holds</Badge> : <Badge tone="facet-u">violated</Badge>],
                ]}
              />
              <h3>Evidence (E)</h3>
              <KV
                rows={[
                  ['source', latest.evidence[0]?.provenance.sourceRef ?? '—'],
                  ['asserted by', latest.evidence[0] === undefined ? '—' : `${latest.evidence[0].provenance.assertedBy.kind}:${latest.evidence[0].provenance.assertedBy.id}`],
                  ['method', `${latest.evidence[0]?.method.ref}@${latest.evidence[0]?.method.version}`],
                  ['observed at', latest.evidence[0]?.temporal.observedAt ?? '—'],
                  ['recorded at', latest.evidence[0]?.temporal.recordedAt ?? '—'],
                  ['quality', `${latest.evidence[0]?.quality.directness}/${latest.evidence[0]?.quality.completeness}/${latest.evidence[0]?.quality.validity}`],
                ]}
              />
              <h3>Uncertainty (U)</h3>
              <ul className="dense">
                {latest.uncertainty.notes.map((note) => (
                  <li key={note.id}>
                    <Badge tone="facet-u">{note.level}</Badge> <span className="mono">{note.code}</span> — {note.detail}
                    {note.mitigation === undefined ? null : <div className="panel-note">mitigation: {note.mitigation}</div>}
                  </li>
                ))}
              </ul>
            </>
          )}
        </Panel>

        <Panel title="Decomposition queue" subtitle="S1 candidates" tight>
          <table>
            <thead>
              <tr>
                <th>signal</th>
                <th>question</th>
                <th>state</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {props.api.view.signals.map((signal) => {
                const decomposed = props.api.view.representations.some((representation) => representation.signalId === signal.id);
                return (
                  <tr key={signal.id}>
                    <td className="mono">{signal.id.slice(0, 14)}…</td>
                    <td>{signal.instrument.itemRef}</td>
                    <td>{decomposed ? <Badge tone="facet-e">representation exists</Badge> : <Badge>“S0 only”</Badge>}</td>
                    <td>
                      {decomposed ? null : (
                        <button type="button" className="action" onClick={() => void props.api.decompose(signal.id)}>
                          decompose
                        </button>
                      )}
                    </td>
                  </tr>
                );
              })}
              {props.api.view.signals.length === 0 ? (
                <tr>
                  <td colSpan={4} className="panel-note">
                    Nothing captured yet.
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
          {openTextSignals.length === 0 ? null : (
            <p className="panel-note" style={{ marginTop: '10px' }}>
              {openTextSignals.length} free-text observation(s) are stored verbatim. Any interpretation of them is a proposal on the record,
              not an observation.
            </p>
          )}
        </Panel>
      </div>
    </div>
  );
}
