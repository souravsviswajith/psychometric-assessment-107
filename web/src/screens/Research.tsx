import { useState, type ReactNode } from 'react';
import type { ExperimentResult } from '../api/types.ts';
import { Badge, Notice, Panel, Rejections, Stat } from '../components/atoms.tsx';
import type { SessionApi } from '../state/runtime.ts';

export function ResearchScreen(props: { api: SessionApi }): ReactNode {
  const [results, setResults] = useState<readonly ExperimentResult[] | undefined>(undefined);
  const [reproducible, setReproducible] = useState<boolean | undefined>(undefined);
  const [report, setReport] = useState<string>('');
  const [running, setRunning] = useState(false);
  const [error, setError] = useState<string | undefined>(undefined);

  const run = async (): Promise<void> => {
    setRunning(true);
    setError(undefined);
    try {
      const run = await props.api.runResearch();
      setResults(run.results);
      setReproducible(run.reproducibility.reproducible);
      setReport(run.report);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : String(caught));
    } finally {
      setRunning(false);
    }
  };

  return (
    <div className="stack">
      <Rejections items={props.api.rejections} />

      <Panel title="Research LOOP" subtitle="Observe → Formalize → Transform → Verify → Execute → Compare → Measure → Refine" tight>
        <p className="panel-note">
          These experiments run the real Go pipeline on this device: capture → capability → decompose → capability → map. They test the
          project's architectural hypothesis that <span className="formula">raw → (A, E, C) | U</span> preserves information that{' '}
          <span className="formula">raw → scalar</span> destroys. They are not evidence for a psychometric claim.
        </p>
        <div className="actions">
          <button type="button" className="action primary" disabled={running} onClick={() => void run()}>
            {running ? 'running experiments…' : 'run the four 1:3 experiments'}
          </button>
          {reproducible === undefined ? null : (
            <span className="panel-note mono">
              fixture reproducibility: {reproducible ? 'identical identities across two runs' : 'NOT reproducible'}
            </span>
          )}
        </div>
        {error === undefined ? null : <Notice tone="alert">{error}</Notice>}
      </Panel>

      {results === undefined ? (
        <Panel title="What will be measured" subtitle="experiment definitions" tight>
          <ul className="dense">
            <li>
              <strong>A — information preservation.</strong> Enumerate facts from the raw observations, then measure how many survive in the 1:3
              representation versus a scalar mean.
            </li>
            <li>
              <strong>B — provenance recovery.</strong> Render each representation with and without the evidence facet and count how many
              provenance questions remain answerable.
            </li>
            <li>
              <strong>C — context dependence.</strong> Hold the assessment constant, thin the context, and compare requirement statuses and
              uncertainty.
            </li>
            <li>
              <strong>D — temporal stability.</strong> Compare the same items across two sittings while tracking assessment, evidence, and
              context separately.
            </li>
          </ul>
        </Panel>
      ) : (
        <>
          {results.map((result) => (
            <Panel
              key={result.id}
              title={`Experiment ${result.id} — ${result.title}`}
              subtitle={`status: ${result.status}`}
              tight
            >
              <p className="panel-note">{result.hypothesis}</p>
              <h3>Method</h3>
              <p className="panel-note">{result.method}</p>
              <h3>Measures</h3>
              <table>
                <thead>
                  <tr>
                    <th>measure</th>
                    <th>value</th>
                    <th>note</th>
                  </tr>
                </thead>
                <tbody>
                  {result.measures.map((measure) => (
                    <tr key={measure.label}>
                      <td>{measure.label}</td>
                      <td className="num">
                        {String(measure.value)}
                        {measure.unit === undefined ? '' : ` ${measure.unit}`}
                      </td>
                      <td className="panel-note">{measure.note ?? ''}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
              <h3>Observations</h3>
              <ul className="dense">
                {result.observations.map((observation) => (
                  <li key={observation}>{observation}</li>
                ))}
              </ul>
              <h3>Limitations</h3>
              <ul className="dense">
                {result.limitations.map((limitation) => (
                  <li key={limitation}>{limitation}</li>
                ))}
              </ul>
              <p className="panel-note mono">fixture: {result.fixtureRef} · ran at {result.ranAt}</p>
            </Panel>
          ))}

          <Panel title="Generated report" subtitle="markdown, produced from the run" tight>
            <pre>{report}</pre>
          </Panel>
        </>
      )}

      <Panel title="Invariant registry" subtitle="what the system checks, and where" tight>
        <table>
          <thead>
            <tr>
              <th>invariant</th>
              <th>scope</th>
              <th>statement</th>
            </tr>
          </thead>
          <tbody>
            {props.api.catalog.invariantDescriptors.map((descriptor) => (
              <tr key={descriptor.id}>
                <td>
                  <Badge tone={descriptor.mandatory ? 'facet-e' : undefined}>{descriptor.id}</Badge>
                </td>
                <td className="mono">{descriptor.scope}</td>
                <td>{descriptor.statement}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </Panel>

      <Panel title="Session state (projection)" subtitle="derived from domain state, not the model" tight>
        <div className="grid cols-4">
          <Stat label="signals" value={props.api.view.projection.signalCount} />
          <Stat label="representations" value={props.api.view.projection.representationCount} />
          <Stat label="transformations" value={props.api.view.projection.transformationCount} />
          <Stat label="executions" value={props.api.view.projection.executionCount} />
        </div>
      </Panel>
    </div>
  );
}
