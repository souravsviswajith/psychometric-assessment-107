import type { ReactNode } from 'react';
import type { PathwayCandidate } from '../api/types.ts';
import { Badge, KV, Notice, Panel, Rejections, Stat } from '../components/atoms.tsx';
import type { SessionApi } from '../state/runtime.ts';

function statusTone(status: PathwayCandidate['requirements'][number]['status']): string {
  return `status-${status}`;
}

function PathwayCard(props: { pathway: PathwayCandidate; api: SessionApi }): ReactNode {
  const { pathway, api } = props;
  return (
    <details>
      <summary>
        <strong>{pathway.name}</strong> · <span className="mono">{pathway.pathwayId}</span> ·{' '}
        <Badge tone="status-supported">{pathway.supportSummary.supported} supported</Badge>{' '}
        <Badge tone="status-gap">{pathway.supportSummary.gap} gap</Badge>{' '}
        <Badge tone="status-conflicting">{pathway.supportSummary.conflicting} conflicting</Badge>{' '}
        <Badge>{pathway.supportSummary.undetermined} undetermined</Badge>{' '}
        {pathway.review.state === 'unreviewed' ? null : <Badge tone="facet-e">{pathway.review.state} (your decision)</Badge>}
      </summary>
      <div className="body stack">
        <p className="panel-note">{pathway.description}</p>

        <table>
          <thead>
            <tr>
              <th>requirement</th>
              <th>construct</th>
              <th>status</th>
              <th>rationale</th>
              <th>observations / evidence</th>
            </tr>
          </thead>
          <tbody>
            {pathway.requirements.map((requirement) => (
              <tr key={requirement.requirementId}>
                <td>{requirement.statement}</td>
                <td className="mono">{requirement.constructId}</td>
                <td>
                  <Badge tone={statusTone(requirement.status)}>{requirement.status}</Badge>
                </td>
                <td className="panel-note">{requirement.rationale}</td>
                <td className="mono panel-note">
                  {requirement.observations.length === 0
                    ? 'none'
                    : requirement.observations
                        .map((observation) => `${observation.value} [${observation.evidenceRefs.join(', ')}]`)
                        .join(' · ')}
                </td>
              </tr>
            ))}
          </tbody>
        </table>

        <div className="grid cols-2">
          <div>
            <h3>Entry routes</h3>
            <ul className="dense">
              {pathway.entryRoutes.map((route) => (
                <li key={route}>{route}</li>
              ))}
            </ul>
            <h3>Trade-offs</h3>
            <ul className="dense">
              {pathway.tradeoffs.map((tradeoff) => (
                <li key={tradeoff.statement}>
                  {tradeoff.statement} <span className="panel-note">— costs: {tradeoff.costsWhat}</span>
                </li>
              ))}
            </ul>
          </div>
          <div>
            <h3>Open questions for you</h3>
            <ul className="dense">
              {pathway.openQuestions.map((question) => (
                <li key={question}>{question}</li>
              ))}
            </ul>
            <h3>Uncertainty carried into this pathway</h3>
            <ul className="dense">
              {pathway.uncertainty.slice(0, 4).map((note) => (
                <li key={note.id}>
                  <Badge tone="facet-u">{note.level}</Badge> <span className="mono">{note.code}</span>
                </li>
              ))}
            </ul>
          </div>
        </div>

        <KV
          rows={[
            ['supporting representations', <span className="mono">{pathway.supportingRepresentationRefs.join(', ')}</span>],
            ['supporting transformations', <span className="mono">{pathway.supportingTransformationRefs.join(' · ')}</span>],
            ['evidence references', <span className="mono">{pathway.evidenceRefs.length} record(s)</span>],
            ['template source', pathway.sourceRef],
            ['your review', `${pathway.review.state} (${pathway.review.decidedBy.kind}:${pathway.review.decidedBy.id})`],
          ]}
        />

        <div className="actions">
          <button type="button" className="action primary" onClick={() => void api.reviewPathway(pathway.pathwayId, 'pursuing')}>
            I want to pursue this
          </button>
          <button type="button" className="action" onClick={() => void api.reviewPathway(pathway.pathwayId, 'interesting')}>
            worth exploring
          </button>
          <button type="button" className="action" onClick={() => void api.reviewPathway(pathway.pathwayId, 'deferred')}>
            not now
          </button>
          <button type="button" className="action" onClick={() => void api.reviewPathway(pathway.pathwayId, 'not-a-fit')}>
            not a fit for me
          </button>
        </div>
        <p className="panel-note">
          These buttons record <em>your</em> decision. The system has no path that sets this state by itself (§2).
        </p>
      </div>
    </details>
  );
}

export function PathwaysScreen(props: { api: SessionApi }): ReactNode {
  const sets = props.api.view.pathwaySets;
  const latest = sets.at(-1);

  return (
    <div className="stack">
      <Rejections items={props.api.rejections} />
      <Panel title="Pathway information" subtitle="M(A, E, C, U) → { P₁ … Pₙ }" tight>
        <div className="actions">
          <button
            type="button"
            className="action primary"
            disabled={props.api.view.representations.length === 0 || props.api.busy}
            onClick={() => void props.api.mapPathways()}
          >
            Map pathways from {props.api.view.representations.length} representation(s)
          </button>
          {latest === undefined ? null : <span className="panel-note mono">set {latest.id}</span>}
        </div>

        <div className="notice signal" style={{ marginTop: '10px' }}>
          {props.api.catalog.userAuthorityStatement}
        </div>

        {latest === undefined ? (
          <Notice tone="warn" >
            No pathway set yet. Decompose at least one observation first; mapping refuses to run without a validated S1 representation.
          </Notice>
        ) : (
          <>
            <div className="grid cols-4" style={{ marginTop: '12px' }}>
              <Stat label="candidates" value={latest.pathways.length} />
              <Stat label="your decisions" value={(() => { const summary = props.api.view.decisionSummaries[latest.id]; return summary === undefined ? 0 : summary.explored + summary.declined + summary.pursuing; })()} />
              <Stat label="constructs assessed" value={latest.coverage.assessedConstructs.length} />
              <Stat label="constructs not assessed" value={latest.coverage.unassessedConstructs.length} />
            </div>

            <h3>Restricted use</h3>
            <ul className="dense">
              {latest.restrictedUse.map((restriction) => (
                <li key={restriction}>{restriction}</li>
              ))}
            </ul>

            <h3>Mapping provenance</h3>
            <KV
              rows={[
                ['mapping transformation', latest.mappingRef],
                ['mapping policy', latest.mappingPolicyRef],
                ['schema version', latest.schemaVersion],
                ['created at', latest.createdAt],
                ['ordering', 'none — the candidates carry no rank, score, or implicit recommendation'],
              ]}
            />
          </>
        )}
      </Panel>

      {latest === undefined ? null : (
        <Panel title="Candidates" subtitle="read them as information, not as a recommendation" tight>
          <div className="stack">
            {latest.pathways.map((pathway) => (
              <PathwayCard key={pathway.id} pathway={pathway} api={props.api} />
            ))}
          </div>
        </Panel>
      )}

      {sets.length <= 1 ? null : (
        <Panel title="Earlier pathway sets" subtitle="previous mappings remain readable" tight>
          <table>
            <thead>
              <tr>
                <th>set</th>
                <th>created</th>
                <th>candidates</th>
                <th>policy</th>
              </tr>
            </thead>
            <tbody>
              {sets
                .slice(0, -1)
                .reverse()
                .map((set) => (
                  <tr key={set.id}>
                    <td className="mono">{set.id.slice(0, 16)}…</td>
                    <td className="mono">{set.createdAt}</td>
                    <td className="num">{set.pathways.length}</td>
                    <td className="mono">{set.mappingPolicyRef}</td>
                  </tr>
                ))}
            </tbody>
          </table>
        </Panel>
      )}
    </div>
  );
}
