import type { ReactNode } from 'react';
import type { SessionApi } from '../state/runtime.ts';
import { Badge, KV, Notice, Panel } from '../components/atoms.tsx';

const STAGES: readonly (readonly [string, string, string])[] = [
  ['S0', 'captured semantic input', 'career.assessment.capture-signal — validated, sized, and bound to an identity'],
  ['S1', 'validated 1:3 representation', 'career.assessment.decompose-1to3 — A/E/C identity carried from the signal'],
  ['S2', 'constrained plan', 'authorization decision, transformation version, limits, declared constraints'],
  ['S3', 'platform representation', 'digest-verified payload artifact + runtime invocation with hard limits'],
  ['S4', 'executed state', 'execution record, result artifact, digest verification, transformation record'],
];

const BOUNDARIES: readonly (readonly [string, string])[] = [
  ['0 · User', 'Origin of intent and the decision authority. Supplies observations, reviews interpretations, decides. Nothing in the system sets a decision on the user’s behalf.'],
  ['1:3 · Infrastructure', 'The representation: x_i → (A_i, E_i, C_i) | U_i — assessment, evidence, career context, with cross-cutting uncertainty.'],
  ['107 · Semantics', 'Contracts, transformations, assessment/evidence/pathway semantics, invariants, versioning, records. No infrastructure concerns.'],
  ['Cloud107 · Infrastructure', 'Workload execution, scheduling, artifact handling, execution state. Receives capabilities and payloads, never career semantics.'],
];

export function ArchitectureScreen(props: { api: SessionApi }): ReactNode {
  const catalog = props.api.catalog;
  const version = (name: string): string => catalog.contractVersions.find((entry) => entry.name === name)?.version ?? '—';
  return (
    <div className="stack">
      <Panel title="The boundary" subtitle="0_User → 1:3_Infrastructure → Information → 0_User" tight>
        <div className="notice signal formula">
          0_User → 1:3_Infrastructure → Information → 0_User
        </div>
        <div className="grid cols-2" style={{ marginTop: '12px' }}>
          {BOUNDARIES.map(([name, description]) => (
            <div key={name} className="notice">
              <strong>{name}</strong>
              <div className="panel-note" style={{ marginTop: '4px' }}>
                {description}
              </div>
            </div>
          ))}
        </div>
      </Panel>

      <Panel title="Pipeline" subtitle="I_t → S0 → S1 → S2 → S3 → S4 → O_t → I_t+1" tight>
        <table>
          <thead>
            <tr>
              <th>stage</th>
              <th>meaning</th>
              <th>implementation in this repository</th>
            </tr>
          </thead>
          <tbody>
            {STAGES.map(([stage, meaning, implementation]) => (
              <tr key={stage}>
                <td className="mono">{stage}</td>
                <td>{meaning}</td>
                <td className="panel-note mono">{implementation}</td>
              </tr>
            ))}
          </tbody>
        </table>
        <p className="panel-note" style={{ marginTop: '10px' }}>
          Stage traces are recorded for every operation, so the path taken is inspectable rather than assumed. The runtime refuses to execute
          a capability that has not been authorized ({`Capability → Authorization → S1 → S2 → Execution → S4`}).
        </p>
      </Panel>

      <Panel title="Capability contracts" subtitle={`version ${version('capabilityContracts')}`} tight>
        <table>
          <thead>
            <tr>
              <th>capability</th>
              <th>scopes</th>
              <th>model</th>
              <th>transformation</th>
              <th>user boundary</th>
            </tr>
          </thead>
          <tbody>
            {catalog.capabilityContracts.map((contract) => (
              <tr key={contract.id}>
                <td className="mono">{contract.id}</td>
                <td className="mono">{contract.requiredScopes.join(', ')}</td>
                <td>{contract.requiresModel ? <Badge tone="facet-u">model-dependent</Badge> : <Badge tone="facet-e">deterministic</Badge>}</td>
                <td className="mono">{contract.transformationRef ?? '—'}</td>
                <td className="panel-note">{contract.userBoundary}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </Panel>

      <div className="grid cols-2">
        <Panel title="Contract versions" subtitle="a result is reproducible from these" tight>
          <KV rows={catalog.contractVersions.map((entry) => [entry.name, <span className="mono">{entry.version}</span>] as const)} />
        </Panel>

        <Panel title="What the system will not build" subtitle="§4, §7, §18" tight>
          <ul className="dense">
            <li>a universal career score — forbidden field names are structurally rejected: {catalog.forbiddenScoreKeys.slice(0, 6).join(', ')}, …</li>
            <li>a deterministic "best career", ranking, or opaque recommendation</li>
            <li>personality diagnosis without validated methodology</li>
            <li>LLM-only assessment scoring, and model output used as evidence</li>
            <li>ads, monetisation, gamification, microservices for appearance</li>
            <li>hidden autonomous decisions of any kind</li>
          </ul>
        </Panel>
      </div>

      <div className="grid cols-2">
        <Panel title="Construct vocabulary" subtitle={`${catalog.vocabularyId}@${version('pathwaySetSchema')}`} tight>
          <Notice tone="warn">
            The construct catalogue is a <strong>project vocabulary</strong>, not a validated psychometric taxonomy. Every instrument item is
            marked <span className="mono">validated: false</span> unless a validation reference is recorded, and every representation carries the
            resulting uncertainty note.
          </Notice>
        </Panel>

        <Panel title="Uncertainty vocabulary" subtitle={`${catalog.uncertaintyCodes.length} registered codes`} tight>
          <div className="row">
            {catalog.uncertaintyCodes.map((code) => (
              <Badge key={code} tone="facet-u">
                {code}
              </Badge>
            ))}
          </div>
          <p className="panel-note" style={{ marginTop: '8px' }}>
            Every note carries a source, kind (epistemic/aleatoric), affected dimensions, a level with a registry floor, and a mitigation.
            Uncertainty is preserved across transformations and can never be silently dropped.
          </p>
        </Panel>
      </div>

      <Panel title="Invariants" subtitle={`${catalog.invariantDescriptors.length} registered, ${catalog.invariantDescriptors.filter((item) => item.mandatory).length} mandatory`} tight>
        <table>
          <thead>
            <tr>
              <th>id</th>
              <th>scope</th>
              <th>statement</th>
            </tr>
          </thead>
          <tbody>
            {catalog.invariantDescriptors.map((descriptor) => (
              <tr key={descriptor.id}>
                <td className="mono">{descriptor.id}</td>
                <td className="mono">{descriptor.scope}</td>
                <td>{descriptor.statement}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </Panel>
    </div>
  );
}
