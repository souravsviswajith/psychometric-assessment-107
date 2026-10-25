import { useState, type ReactNode } from 'react';
import type { SessionApi } from '../state/runtime.ts';
import { CaptureScreen } from '../screens/Capture.tsx';
import { RepresentationsScreen } from '../screens/Representations.tsx';
import { PathwaysScreen } from '../screens/Pathways.tsx';
import { ResearchScreen } from '../screens/Research.tsx';
import { ArchitectureScreen } from '../screens/Architecture.tsx';
import { Badge, Notice, Panel, Rejections, Stat } from '../components/atoms.tsx';

const TABS = ['overview', 'capture', 'representations', 'pathways', 'research', 'architecture'] as const;
type Tab = (typeof TABS)[number];

/**
 * The technical projection: the original inspection screens (counters,
 * identities, facets, research, architecture). Reached from the app's
 * "You" tab for reviewers who want to see the 1:3 machinery.
 */
export function TechnicalView({ api, onClose }: { api: SessionApi; onClose: () => void }): ReactNode {
  const [tab, setTab] = useState<Tab>('overview');
  const projection = api.view.projection;

  return (
    <div className="app">
      <header className="masthead">
        <div>
          <button type="button" className="action" style={{ marginBottom: '10px' }} onClick={onClose}>
            ← back to the app
          </button>
          <h1>Career Assessment 107 · technical view</h1>
          <p className="boundary">
            0<sub>User</sub> → 1:3<sub>Infrastructure</sub> → Information → 0<sub>User</sub> · <strong>the user decides</strong>
          </p>
        </div>
        <div className="masthead-meta">
          representation 1.3.0 · uncertainty profile 1.0.0 · pathway 1.0.0
          <br />
          subject {String(projection.subjectId)}
          <br />
          runtime {api.catalog.runtimeId} · <button type="button" className="action" style={{ marginLeft: '6px' }} disabled={api.busy} onClick={() => void api.reset()}>reset session</button>
        </div>
      </header>

      <nav className="tabs" aria-label="Sections">
        {TABS.map((candidate) => (
          <button key={candidate} type="button" aria-current={tab === candidate} onClick={() => setTab(candidate)}>
            {candidate}
          </button>
        ))}
      </nav>

      {api.notice === undefined ? null : (
        <Notice tone="signal">
          {api.notice}{' '}
          <button type="button" className="action" style={{ marginLeft: '8px' }} onClick={api.clearMessages}>
            dismiss
          </button>
        </Notice>
      )}
      <Rejections items={api.rejections} />

      <div style={{ marginTop: '14px' }}>
        {tab === 'overview' ? <Overview api={api} goto={setTab} /> : null}
        {tab === 'capture' ? <CaptureScreen api={api} /> : null}
        {tab === 'representations' ? <RepresentationsScreen api={api} /> : null}
        {tab === 'pathways' ? <PathwaysScreen api={api} /> : null}
        {tab === 'research' ? <ResearchScreen api={api} /> : null}
        {tab === 'architecture' ? <ArchitectureScreen api={api} /> : null}
      </div>

      <footer className="panel" style={{ marginTop: '18px' }}>
        <p className="panel-note">
          {api.catalog.userAuthorityStatement} {api.catalog.noScalarCollapseStatement}
        </p>
      </footer>
    </div>
  );
}

function Overview(props: { api: SessionApi; goto: (tab: Tab) => void }): ReactNode {
  const { projection, representations, pathwaySets, decisions } = props.api.view;
  const items = props.api.catalog.items;
  const constructs = new Set(representations.flatMap((representation) => representation.A.constructs.map((construct) => construct.constructId)));

  return (
    <div className="stack">
      <Panel title="Where this session stands" subtitle="derived state · UI ≠ truth" tight>
        <div className="grid cols-4">
          <Stat label="observations (S0)" value={projection.signalCount} hint="what you supplied" />
          <Stat label="representations (S1)" value={projection.representationCount} hint="(A, E, C) with U" />
          <Stat label="pathway sets" value={projection.pathwaySetCount} hint="information, not a recommendation" />
          <Stat label="your decisions" value={projection.decisionCount} hint="recorded against your name, not the system’s" />
        </div>
        <div className="grid cols-4" style={{ marginTop: '12px' }}>
          <Stat label="transformations" value={projection.transformationCount} />
          <Stat label="executions" value={projection.executionCount} />
          <Stat label="uncertainty notes" value={projection.uncertaintyNotes} />
          <Stat label="proposals" value={projection.proposalCount} />
        </div>
      </Panel>

      <div className="grid cols-2">
        <Panel title="Start here" subtitle="the smallest coherent step" tight>
          <ol className="dense">
            <li>
              <strong>Capture</strong> an observation: answer one item, with the context you are answering from. Buttons: <Badge tone="facet-a">capture</Badge>
            </li>
            <li>
              <strong>Decompose</strong> it: the signal becomes (A, E, C) with cross-cutting uncertainty, through the capability boundary.
            </li>
            <li>
              <strong>Read</strong> the representation: what was asked, how it was answered, where it came from, how uncertain it is.
            </li>
            <li>
              <strong>Map</strong> pathways once at least one representation exists; then <strong>review</strong> them yourself.
            </li>
          </ol>
          <div className="actions">
            <button type="button" className="action primary" onClick={() => props.goto('capture')}>
              go to capture
            </button>
            <button
              type="button"
              className="action"
              disabled={representations.length === 0 || props.api.busy}
              onClick={() => void props.api.mapPathways()}
            >
              map pathways
            </button>
            <button type="button" className="action" onClick={() => props.goto('research')}>
              verify the architecture
            </button>
          </div>
        </Panel>

        <Panel title="Session inventory" subtitle="what has been observed so far" tight>
          <table>
            <tbody>
              <tr>
                <td className="mono">instrument items available</td>
                <td className="num">{items.length}</td>
              </tr>
              <tr>
                <td className="mono">constructs observed</td>
                <td className="num">{constructs.size}</td>
              </tr>
              <tr>
                <td className="mono">pathway candidates available</td>
                <td className="num">{pathwaySets.at(-1)?.pathways.length ?? 0}</td>
              </tr>
              <tr>
                <td className="mono">decisions recorded</td>
                <td className="num">{decisions.length}</td>
              </tr>
              <tr>
                <td className="mono">last activity</td>
                <td className="mono">{projection.lastActivityAt}</td>
              </tr>
            </tbody>
          </table>
          <p className="panel-note" style={{ marginTop: '10px' }}>
            {props.api.view.persisted
              ? 'The Go runtime runs on this device and saves the session here. Nothing is transmitted anywhere; the runtime, the domain, and the research experiments all execute locally.'
              : 'The Go runtime keeps this session in memory. Nothing is transmitted anywhere; the runtime, the domain, and the research experiments all execute locally.'}
          </p>
        </Panel>
      </div>

      <Panel title="What this system is" subtitle="and what it refuses to be" tight>
        <div className="grid cols-2">
          <div>
            <h3>It is</h3>
            <ul className="dense">
              <li>a traceable, versioned, evidence-aware career-information system</li>
              <li>a representation of observations you supplied, with their provenance and uncertainty</li>
              <li>an explainer of what each career direction would require, and what remains unknown</li>
              <li>a record of your decisions that never rewrites its own history</li>
            </ul>
          </div>
          <div>
            <h3>It is not</h3>
            <ul className="dense">
              <li>a test that tells you what you are</li>
              <li>a scoring engine that ranks careers for you</li>
              <li>a source of psychometric claims: the instrument ships unvalidated and says so</li>
              <li>an authority on your decision — that stays with you</li>
            </ul>
          </div>
        </div>
      </Panel>
    </div>
  );
}
