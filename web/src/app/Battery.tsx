import { useState, type ReactNode } from 'react';
import type { SessionApi } from '../state/runtime.ts';
import { Icon } from './icons.tsx';
import { MODULES, answerText, batteryOrder, buildAnswer, latestAnswers, moduleIndex, selectedOptionId, shortLabel, type Profile } from './shared.tsx';

/**
 * The assessment card: one item at a time with the candidate response
 * matrix. "Save & Continue" captures the answer (S0) and decomposes it into
 * (A, E, C) | U (S1→S4) in one call.
 */
export function Battery(props: { api: SessionApi; profile: Profile; index: number; onIndex: (index: number) => void; onDone: () => void }): ReactNode {
  const { api, profile, index } = props;
  const items = batteryOrder(api.catalog.items);
  const answers = latestAnswers(api.view);
  const item = items[index];
  const [draft, setDraft] = useState<{ itemRef: string; value: string } | undefined>(undefined);

  if (item === undefined) {
    return (
      <section className="c-card">
        <span className="c-tag">Battery complete</span>
        <h2 className="c-h2">All {items.length} items recorded</h2>
        <p className="c-body-sm">Your results dossier is ready. You can revisit any item from the Syllabus.</p>
        <button type="button" className="c-btn c-btn-primary c-btn-block" onClick={props.onDone}>
          View Results Dossier <Icon name="arrow_forward" size={18} />
        </button>
      </section>
    );
  }

  const module = moduleIndex(item);
  const inModule = items.filter((candidate) => moduleIndex(candidate) === module);
  const completion = (inModule.filter((candidate) => answers.has(candidate.itemRef)).length / inModule.length) * 100;
  const current = answers.get(item.itemRef);
  const saved = item.responseShape === 'open-text' ? (current === undefined ? '' : answerText(current)) : selectedOptionId(current, item);
  const value = draft?.itemRef === item.itemRef ? draft.value : (saved ?? '');
  const matrix = item.options.length === 5 && item.responseShape !== 'single-select';

  const save = async (): Promise<void> => {
    const next = (): void => {
      setDraft(undefined);
      const open = items.findIndex((candidate, position) => position > index && !latestAnswers(api.view).has(candidate.itemRef));
      props.onIndex(open < 0 ? items.length : open);
    };
    if (value === saved) return next();
    const response = item.responseShape === 'open-text' ? ({ kind: 'text', text: value.trim() } as const) : ({ kind: 'option', optionId: value } as const);
    if (await api.answer(buildAnswer(item, response, profile))) next();
  };

  return (
    <section className="c-card">
      <div className="c-row">
        <span className="c-tag c-tag-soft">Self-Description Battery v{item.instrumentVersion}</span>
        <span className="c-mono c-caption">
          Item {index + 1} / {items.length}
        </span>
      </div>
      <h2 className="c-h2">
        Module {module + 1}: {MODULES[module]!.title}
      </h2>
      <div className="c-rule" />
      <div>
        <div className="c-row c-caption">
          <span>Section Completion</span>
          <span className="c-mono c-strong">{completion.toFixed(1)}%</span>
        </div>
        <div className="c-bar">
          <div style={{ width: `${completion}%` }} />
        </div>
      </div>
      <div className="c-quote">
        <span className="c-label">Standard Item Formulation</span>
        <p>“{item.promptText}”</p>
      </div>

      <span className="c-label">Candidate Response Matrix</span>
      {item.responseShape === 'open-text' ? (
        <textarea
          className="c-input c-textarea"
          rows={5}
          maxLength={4000}
          value={value}
          placeholder="Write a few sentences in your own words."
          onChange={(event) => setDraft({ itemRef: item.itemRef, value: event.target.value })}
        />
      ) : (
        <div className={matrix ? 'c-matrix' : 'c-choices'} role="radiogroup" aria-label={item.promptText}>
          {item.options.map((option, position) => (
            <button
              key={option.id}
              type="button"
              role="radio"
              aria-checked={value === option.id}
              aria-label={option.label}
              className={value === option.id ? 'is-selected' : ''}
              onClick={() => setDraft({ itemRef: item.itemRef, value: option.id })}
            >
              {matrix ? (
                <>
                  <span className="c-mono c-strong">{position + 1}</span>
                  <small>{shortLabel(option.label)}</small>
                </>
              ) : (
                option.label
              )}
            </button>
          ))}
        </div>
      )}
      {matrix && value !== '' ? <p className="c-caption">Selected: {item.options.find((option) => option.id === value)?.label}</p> : null}

      <div className="c-row c-footer">
        <button type="button" className="c-link" disabled={index === 0} onClick={() => props.onIndex(index - 1)}>
          <Icon name="history" size={16} /> Review Previous
        </button>
        <button type="button" className="c-btn c-btn-primary" disabled={api.busy || value.trim() === ''} onClick={() => void save()}>
          Save &amp; Continue <Icon name="chevron_right" size={18} />
        </button>
      </div>
    </section>
  );
}
