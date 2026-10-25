import { useState, type ReactNode } from 'react';
import type { PathwaySet } from '../api/types.ts';
import type { SessionApi } from '../state/runtime.ts';
import { Icon } from './icons.tsx';
import { DECISIONS, FAMILY_ICON, STATUSES, STATUS_TEXT, byMatch, decisionLabel, matchPercent, plainObservation, plainReason } from './shared.tsx';

export function latestSet(api: SessionApi): PathwaySet | undefined {
  return api.view.pathwaySets.at(-1);
}

/**
 * Results are stale when they were not mapped from the latest answer to each
 * question (an answer was added or changed since).
 */
export function isStale(api: SessionApi): boolean {
  const mapped = latestSet(api)?.pathways[0]?.supportingRepresentationRefs;
  const latest = new Map(api.view.representations.map((representation) => [representation.A.instrument.itemRef, representation.id]));
  return mapped === undefined ? latest.size > 0 : mapped.length !== latest.size || [...latest.values()].some((id) => !mapped.includes(id));
}

/** The "Career Pathways" card: directions with the share of requirements matched. */
export function PathwayList(props: { api: SessionApi; limit?: number; onOpen: (pathwayId: string) => void; onStart: () => void }): ReactNode {
  const set = latestSet(props.api);
  const pathways = set === undefined ? [] : byMatch(set.pathways).slice(0, props.limit);
  return (
    <section className="c-card">
      <div className="c-row">
        <h2 className="c-h2">Career Pathways</h2>
        <span className="c-mono c-caption">{set === undefined ? 'Awaiting assessment' : `${set.pathways.length} Pathways Mapped`}</span>
      </div>
      <p className="c-body-sm">Career directions matched to your assessment profile.</p>
      <div className="c-rule" />
      {set === undefined ? (
        <button type="button" className="c-btn c-btn-secondary c-btn-block" onClick={props.onStart}>
          Answer the battery to map your pathways
        </button>
      ) : (
        <div className="c-stack">
          {pathways.map((pathway, position) => (
            <button key={pathway.id} type="button" className="c-path" onClick={() => props.onOpen(pathway.pathwayId)}>
              <div className="c-row c-top">
                <div className="c-path-head">
                  <span className="c-icon-box">
                    <Icon name={FAMILY_ICON[pathway.family] ?? 'school'} />
                  </span>
                  <div>
                    <h3 className="c-h3">{pathway.name}</h3>
                    <p className="c-caption">
                      {pathway.supportSummary.supported} of {pathway.supportSummary.total} requirements met
                      {decisionLabel(pathway.review.state) === undefined ? '' : ` · ${decisionLabel(pathway.review.state)}`}
                    </p>
                  </div>
                </div>
                <span className={`c-badge c-mono${position === 0 ? ' c-badge-strong' : ''}`}>{matchPercent(pathway)}% Matched</span>
              </div>
              <div className="c-row c-path-foot c-caption">
                <span>Track: {pathway.family.charAt(0).toUpperCase() + pathway.family.slice(1)}</span>
                <span className="c-strong">
                  Explore <Icon name="arrow_forward" size={14} />
                </span>
              </div>
            </button>
          ))}
        </div>
      )}
    </section>
  );
}

export function PathwayDetail(props: { api: SessionApi; pathwayId: string; onBack: () => void }): ReactNode {
  const { api } = props;
  const set = latestSet(api);
  const pathway = set?.pathways.find((candidate) => candidate.pathwayId === props.pathwayId);
  const [why, setWhy] = useState<string | undefined>(undefined);
  if (set === undefined || pathway === undefined) return null;

  return (
    <div className="c-stack-lg">
      <button type="button" className="c-link" onClick={props.onBack}>
        <Icon name="arrow_back" size={16} /> Results Dossier
      </button>
      <section className="c-card">
        <div className="c-row">
          <span className="c-tag">{pathway.family}</span>
          <span className="c-badge c-badge-strong c-mono">{matchPercent(pathway)}% Matched</span>
        </div>
        <h1 className="c-h1">{pathway.name}</h1>
        <p className="c-body-sm">{pathway.description}</p>
        <p className="c-caption">
          {STATUSES.filter((status) => pathway.supportSummary[status] > 0)
            .map((status) => `${pathway.supportSummary[status]} ${STATUS_TEXT[status].short.toLowerCase()}`)
            .join(' · ')}
        </p>
      </section>

      <section className="c-card">
        <h2 className="c-h2">Requirement Ledger</h2>
        <div className="c-rule" />
        {pathway.requirements.map((requirement) => (
          <div key={requirement.requirementId} className={`c-req c-req-${requirement.status}`}>
            <span className="c-status">
              <Icon name={STATUS_TEXT[requirement.status].icon} size={16} /> {STATUS_TEXT[requirement.status].long}
            </span>
            <p className="c-body">{requirement.statement}</p>
            {requirement.observations.length === 0 ? null : (
              <p className="c-caption">Your response: {requirement.observations.map((observation) => plainObservation(observation.value)).join(' · ')}</p>
            )}
            <button type="button" className="c-link" onClick={() => setWhy(why === requirement.requirementId ? undefined : requirement.requirementId)}>
              {why === requirement.requirementId ? 'Hide reasoning' : 'Why?'}
            </button>
            {why === requirement.requirementId ? (
              <>
                <p className="c-body-sm">{plainReason(requirement)}</p>
                {requirement.kind === 'constraint' ? null : <p className="c-caption c-mono">{requirement.rationale}</p>}
              </>
            ) : null}
          </div>
        ))}
      </section>

      {(
        [
          ['Entry Routes', pathway.entryRoutes],
          ['Trade-offs', pathway.tradeoffs.map((tradeoff) => `${tradeoff.statement} It can cost: ${tradeoff.costsWhat.toLowerCase()}`)],
          ['Reflection Questions', pathway.openQuestions],
        ] as const
      ).map(([title, lines]) => (
        <section key={title} className="c-card">
          <h2 className="c-h2">{title}</h2>
          <div className="c-rule" />
          <ul className="c-list">
            {lines.map((line) => (
              <li key={line}>{line}</li>
            ))}
          </ul>
        </section>
      ))}

      <section className="c-card">
        <h2 className="c-h2">Record Your Decision</h2>
        <p className="c-body-sm">The portal never decides for you. Your decision is recorded as yours and can be changed at any time.</p>
        <div className="c-decisions">
          {DECISIONS.map((decision) => (
            <button
              key={decision.state}
              type="button"
              className={pathway.review.state === decision.state ? 'is-selected' : ''}
              disabled={api.busy}
              onClick={() => {
                if (pathway.review.state !== decision.state) void api.reviewPathway(pathway.pathwayId, decision.state);
              }}
            >
              <strong>{decision.label}</strong>
              <small>{decision.hint}</small>
            </button>
          ))}
        </div>
      </section>

      <details className="c-card">
        <summary className="c-h3">Interpretation Notes</summary>
        <ul className="c-list">
          {set.restrictedUse.map((restriction) => (
            <li key={restriction}>{restriction}</li>
          ))}
        </ul>
      </details>
    </div>
  );
}
