import { useState, type ReactNode } from 'react';
import type { SessionApi } from '../state/runtime.ts';
import { Icon } from './icons.tsx';
import {
  MODULES,
  SETTING_OPTIONS,
  STAGE_OPTIONS,
  answerText,
  batteryOrder,
  candidateId,
  decisionLabel,
  formatDate,
  initials,
  labelFor,
  latestAnswers,
  moduleIndex,
  type Profile,
} from './shared.tsx';

/** Candidate details: first-run registration and later edits. */
export function ProfileForm(props: { initial?: Profile; submitLabel: string; onSubmit: (profile: Profile) => void }): ReactNode {
  const [name, setName] = useState(props.initial?.name ?? '');
  const [setting, setSetting] = useState(props.initial?.setting ?? '');
  const [careerStage, setCareerStage] = useState(props.initial?.careerStage ?? '');
  return (
    <div className="c-stack">
      <label className="c-field">
        <span className="c-label">Full Name</span>
        <input className="c-input" value={name} maxLength={60} autoComplete="name" placeholder="e.g. Anjali Nair" onChange={(event) => setName(event.target.value)} />
      </label>
      {(
        [
          ['Current Situation', SETTING_OPTIONS, setting, setSetting],
          ['Career Stage', STAGE_OPTIONS, careerStage, setCareerStage],
        ] as const
      ).map(([legend, options, value, set]) => (
        <fieldset key={legend} className="c-field">
          <legend className="c-label">{legend}</legend>
          <div className="c-pills">
            {options.map((option) => (
              <button key={option.value} type="button" aria-pressed={value === option.value} className={value === option.value ? 'is-active' : ''} onClick={() => set(option.value)}>
                {option.label}
              </button>
            ))}
          </div>
        </fieldset>
      ))}
      <button
        type="button"
        className="c-btn c-btn-primary c-btn-block"
        disabled={name.trim() === '' || setting === '' || careerStage === ''}
        onClick={() => props.onSubmit({ name: name.trim(), setting, careerStage })}
      >
        {props.submitLabel} <Icon name="arrow_forward" size={18} />
      </button>
    </div>
  );
}

export function Registration(props: { questionCount: number; onSubmit: (profile: Profile) => void }): ReactNode {
  return (
    <div className="c-screen c-stack-lg">
      <div className="c-brand">
        <span className="c-seal">
          <Icon name="account_balance" size={22} />
        </span>
        <div>
          <strong>Career Intelligence</strong>
          <small>International University</small>
        </div>
      </div>
      <section className="c-card">
        <span className="c-tag">
          <Icon name="verified" size={14} /> Directorate of Career Intelligence
        </span>
        <h1 className="c-h1">Candidate Registration</h1>
        <p className="c-body-sm">
          Register to begin the {props.questionCount}-item self-description battery. Your responses build a career dossier with pathways matched to
          your profile.
        </p>
        <div className="c-rule" />
        <ProfileForm submitLabel="Register & Proceed" onSubmit={props.onSubmit} />
      </section>
      <p className="c-footnote">
        <Icon name="lock" size={14} /> Your record is stored only on this device.
      </p>
    </div>
  );
}

export function Syllabus(props: { api: SessionApi; onItem: (index: number) => void }): ReactNode {
  const items = batteryOrder(props.api.catalog.items);
  const answers = latestAnswers(props.api.view);
  const instrument = items[0];
  return (
    <div className="c-stack-lg">
      <section className="c-card">
        <span className="c-tag c-tag-soft">Assessment Scope</span>
        <h1 className="c-h1">Self-Description Battery</h1>
        <p className="c-body-sm">
          {items.length} items in {MODULES.length} modules, about 5 minutes. Each response is recorded with its context and evidence, and
          interpreted against the career pathways.
        </p>
        <div className="c-grid-2">
          <div className="c-stat">
            <strong>{answers.size}</strong>
            <span>Items Recorded</span>
          </div>
          <div className="c-stat">
            <strong>v{instrument?.instrumentVersion}</strong>
            <span>Battery Version</span>
          </div>
        </div>
        <p className="c-caption">This draft instrument has not been scientifically validated; treat results as prompts for reflection.</p>
      </section>
      {MODULES.map((module, m) => (
        <section key={module.title} className="c-card">
          <div className="c-row">
            <h2 className="c-h2">
              Module {m + 1}: {module.title}
            </h2>
          </div>
          <div className="c-rule" />
          <div className="c-ledger">
            {items.map((item, index) =>
              moduleIndex(item) !== m ? null : (
                <button key={item.itemRef} type="button" onClick={() => props.onItem(index)}>
                  <span className="c-mono c-caption">{String(index + 1).padStart(2, '0')}</span>
                  <span className="c-body-sm">{item.promptText}</span>
                  <Icon name={answers.has(item.itemRef) ? 'check_circle' : 'chevron_right'} size={18} />
                </button>
              ),
            )}
          </div>
        </section>
      ))}
    </div>
  );
}

export function Portal(props: { api: SessionApi; profile: Profile; onProfile: (profile: Profile) => void; onTechnical: () => void; onReset: () => void }): ReactNode {
  const { api, profile } = props;
  const [editing, setEditing] = useState(false);
  const [confirmReset, setConfirmReset] = useState(false);
  const items = api.catalog.items;
  const answers = latestAnswers(api.view);
  const set = api.view.pathwaySets.at(-1);

  return (
    <div className="c-stack-lg">
      <section className="c-card">
        <div className="c-row">
          <div className="c-path-head">
            <span className="c-avatar">{initials(profile.name)}</span>
            <div>
              <h1 className="c-h2">{profile.name}</h1>
              <span className="c-mono c-caption">Candidate ID: {candidateId(api.view)}</span>
            </div>
          </div>
          {editing ? null : (
            <button type="button" className="c-link" onClick={() => setEditing(true)}>
              <Icon name="edit" size={16} /> Edit
            </button>
          )}
        </div>
        <div className="c-rule" />
        {editing ? (
          <ProfileForm
            initial={profile}
            submitLabel="Save Details"
            onSubmit={(next) => {
              props.onProfile(next);
              setEditing(false);
            }}
          />
        ) : (
          <p className="c-body-sm">
            {labelFor(SETTING_OPTIONS, profile.setting)} · {labelFor(STAGE_OPTIONS, profile.careerStage)}
          </p>
        )}
      </section>

      <section className="c-card">
        <h2 className="c-h2">Decision Record</h2>
        <div className="c-rule" />
        {api.view.decisions.length === 0 ? (
          <p className="c-body-sm">No decisions recorded yet. Open a pathway in your dossier to record one.</p>
        ) : (
          <div className="c-ledger">
            {[...api.view.decisions].reverse().map((decision) => {
              const pathway = set?.pathways.find((candidate) => candidate.pathwayId === decision.targetRef);
              return (
                <div key={decision.id}>
                  <span className="c-body-sm c-strong">{pathway?.name ?? decision.targetRef}</span>
                  <span className="c-caption">
                    {decisionLabel(decision.decisionKind) ?? decision.decisionKind} · {formatDate(decision.decidedAt)}
                  </span>
                </div>
              );
            })}
          </div>
        )}
      </section>

      <section className="c-card">
        <div className="c-row">
          <h2 className="c-h2">Assessment Record</h2>
          <span className="c-mono c-caption">
            {answers.size} / {items.length}
          </span>
        </div>
        <div className="c-rule" />
        <div className="c-ledger">
          {batteryOrder(items)
            .filter((item) => answers.has(item.itemRef))
            .map((item) => (
              <div key={item.itemRef}>
                <span className="c-body-sm">{item.promptText}</span>
                <span className="c-caption">
                  {answerText(answers.get(item.itemRef)!)} · {formatDate(answers.get(item.itemRef)!.capturedAt)}
                </span>
              </div>
            ))}
        </div>
      </section>

      <section className="c-card">
        <h2 className="c-h2">About This Assessment</h2>
        <p className="c-body-sm">{api.catalog.userAuthorityStatement}</p>
        <button type="button" className="c-btn c-btn-secondary c-btn-block" onClick={props.onTechnical}>
          <Icon name="info" size={18} /> Technical Details
        </button>
        {confirmReset ? (
          <div className="c-confirm">
            <p className="c-body-sm">Delete your registration, responses and decisions from this device? This cannot be undone.</p>
            <div className="c-row">
              <button type="button" className="c-btn c-btn-secondary" onClick={() => setConfirmReset(false)}>
                Cancel
              </button>
              <button
                type="button"
                className="c-btn c-btn-danger"
                disabled={api.busy}
                onClick={() => {
                  setConfirmReset(false);
                  props.onReset();
                }}
              >
                Delete Everything
              </button>
            </div>
          </div>
        ) : (
          <button type="button" className="c-btn c-btn-danger-outline c-btn-block" onClick={() => setConfirmReset(true)}>
            Start Over
          </button>
        )}
      </section>
    </div>
  );
}
