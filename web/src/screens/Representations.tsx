import type { ReactNode } from 'react';
import type { RenderedRepresentation } from '../api/types.ts';
import { Badge, KV, Notice, Panel, Rejections, Stat } from '../components/atoms.tsx';
import type { SessionApi } from '../state/runtime.ts';

function FacetRows(props: { rendered: RenderedRepresentation | undefined }): ReactNode {
  const { rendered } = props;
  if (rendered === undefined) return null;
  return (
    <>
      {rendered.sections.map((section) => (
        <details key={section.key}>
          <summary>
            <Badge
              tone={
                section.key === 'A' ? 'facet-a' : section.key === 'E' ? 'facet-e' : section.key === 'C' ? 'facet-c' : 'facet-u'
              }
            >
              {section.key}
            </Badge>{' '}
            {section.title}
          </summary>
          <div className="body">
            <table>
              <tbody>
                {section.rows.map((row, index) => (
                  <tr key={`${section.key}-${index}-${row.label}`}>
                    <td className="mono" style={{ width: '30%' }}>
                      {row.label}
                    </td>
                    <td>{row.value}</td>
                    <td className="mono panel-note" style={{ width: '22%' }}>
                      {row.provenance ?? ''}
                      {row.detail === undefined ? '' : ` · ${row.detail}`}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </details>
      ))}
    </>
  );
}

export function RepresentationsScreen(props: { api: SessionApi }): ReactNode {
  const { representations, corrections, proposals } = props.api.view;

  return (
    <div className="stack">
      <Rejections items={props.api.rejections} />

      <Panel
        title="1:3 representations"
        subtitle="S1 · x_i → (A_i, E_i, C_i) | U_i"
        tight
      >
        <div className="grid cols-4">
          <Stat label="representations" value={representations.length} />
          <Stat label="observations" value={props.api.view.signals.length} />
          <Stat label="corrections" value={corrections.length} />
          <Stat label="proposals" value={proposals.length} />
        </div>
        <p className="panel-note" style={{ marginTop: '10px' }}>
          One observation yields three representations and cross-cutting uncertainty. There is no combined number here, because there is no
          combined number in the architecture: a single observation is not a career conclusion.
        </p>

        {representations.length === 0 ? (
          <Notice>
            No representations yet. Capture an observation on the <strong>Capture</strong> tab and decompose it.
          </Notice>
        ) : (
          representations.map((representation) => (
            <details key={representation.id} style={{ marginTop: '10px' }}>
              <summary>
                {representation.A.constructs.map((construct) => construct.label).join(', ')} — {representation.A.setting} ·{' '}
                <span className="mono">{representation.signalId}</span>
              </summary>
              <div className="body stack">
                <KV
                  rows={[
                    ['representation.id', <span className="mono">{representation.id}</span>],
                    ['schema version', representation.schemaVersion],
                    ['transformation', representation.transformationRef],
                    ['identity', `A.id = E.id = C.id = ${representation.signalId.slice(0, 16)}…`],
                    ['question', representation.A.instrument.promptText],
                    ['answer', JSON.stringify(representation.A.response).slice(0, 160)],
                    ['recorded by', `${representation.createdBy.kind}:${representation.createdBy.id}`],
                  ]}
                />
                <FacetRows rendered={props.api.view.rendered[representation.id]} />
                <div className="actions">
                  <button type="button" className="action" onClick={() => void props.api.proposeQuestions(representation.id)}>
                    propose follow-up questions (model, marked as proposal)
                  </button>
                  <button
                    type="button"
                    className="action"
                    onClick={() =>
                      void props.api.correctRepresentation(
                        representation.id,
                        `corrected in the app at ${new Date().toISOString()}`,
                      )
                    }
                  >
                    correct context (adds a correction record)
                  </button>
                </div>
              </div>
            </details>
          ))
        )}
      </Panel>

      {proposals.length === 0 ? null : (
        <Panel title="Model proposals" subtitle="S3 · proposals, never evidence" tight>
          <table>
            <thead>
              <tr>
                <th>proposal</th>
                <th>kind</th>
                <th>provider / model</th>
                <th>instruction</th>
                <th>status</th>
                <th>uncertainty</th>
              </tr>
            </thead>
            <tbody>
              {proposals.map((proposal) => (
                <tr key={proposal.id}>
                  <td className="mono">{proposal.id.slice(0, 14)}…</td>
                  <td>{proposal.proposalKind}</td>
                  <td className="mono">
                    {proposal.modelMetadata.provider}/{proposal.modelMetadata.model}@{proposal.modelMetadata.modelVersion}
                  </td>
                  <td className="mono">{proposal.modelMetadata.instructionVersion}</td>
                  <td>
                    <Badge tone={proposal.status === 'proposed' ? 'facet-u' : 'facet-e'}>{proposal.status}</Badge>
                  </td>
                  <td className="mono">{proposal.uncertainty.map((note) => note.code).join(', ')}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {proposals.map((proposal) => (
            <div key={`${proposal.id}-text`} className="notice warn" style={{ marginTop: '8px' }}>
              <div className="panel-note mono">model output · evidenceRefs: {proposal.evidenceRefs.length}</div>
              {proposal.summary}
              <div className="panel-note" style={{ marginTop: '6px' }}>
                {proposal.modelMetadata.uncertainty}
              </div>
            </div>
          ))}
        </Panel>
      )}

      {corrections.length === 0 ? null : (
        <Panel title="Corrections" subtitle="history is appended, never rewritten" tight>
          <table>
            <thead>
              <tr>
                <th>correction</th>
                <th>target</th>
                <th>dimension</th>
                <th>from → to</th>
                <th>reason</th>
              </tr>
            </thead>
            <tbody>
              {corrections.map((correction) => (
                <tr key={correction.id}>
                  <td className="mono">{correction.id.slice(0, 14)}…</td>
                  <td className="mono">{correction.targetRef.slice(0, 16)}…</td>
                  <td>
                    <Badge tone="facet-c">{correction.dimension}</Badge>
                  </td>
                  <td>
                    {correction.previousValueSummary} → {correction.correctedValueSummary}
                  </td>
                  <td>{correction.reason}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </Panel>
      )}
    </div>
  );
}
