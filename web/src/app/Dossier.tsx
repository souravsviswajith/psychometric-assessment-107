import type { ReactNode } from 'react';
import type { SessionApi } from '../state/runtime.ts';
import { Icon } from './icons.tsx';
import { latestSet } from './Pathways.tsx';
import { byMatch, candidateId, initials, latestAnswers, strengths, type Profile } from './shared.tsx';

/** Saves the page as a PDF: through Android's print service in the app, the browser's otherwise. */
function downloadDossier(): void {
  const native = (window as unknown as { Career107Native?: { print: () => void } }).Career107Native;
  if (native === undefined) window.print();
  else native.print();
}

/** The candidate snapshot: identity, primary alignment, strongest self-ratings. */
export function DossierCard(props: { api: SessionApi; profile: Profile }): ReactNode {
  const { api, profile } = props;
  const items = api.catalog.items;
  const answered = latestAnswers(api.view).size;
  const primary = byMatch(latestSet(api)?.pathways ?? [])[0];
  const top = strengths(api.view, items);

  return (
    <section className="c-card c-dossier">
      <div className="c-row">
        <div className="c-path-head">
          <span className="c-avatar">{initials(profile.name)}</span>
          <div>
            <h2 className="c-h2">{profile.name === '' ? 'Candidate' : profile.name}</h2>
            <span className="c-mono c-caption">Candidate ID: {candidateId(api.view)}</span>
          </div>
        </div>
        <span className="c-badge">{answered === items.length ? 'Complete' : `${answered}/${items.length} Items`}</span>
      </div>
      <div className="c-rule" />
      <div className="c-quote">
        <span className="c-label">Primary Alignment Archetype</span>
        {primary === undefined ? (
          <p className="c-body-sm">Answer the battery to see the career direction your profile aligns with most.</p>
        ) : (
          <>
            <h3 className="c-h3 c-serif">{primary.name}</h3>
            <p className="c-body-sm">{primary.description}</p>
          </>
        )}
      </div>
      <span className="c-label">Top Self-Rated Strengths</span>
      {top.length === 0 ? (
        <p className="c-caption">Appears once you have answered the interest, value, style and skill items.</p>
      ) : (
        top.map((strength, position) => (
          <div key={strength.label}>
            <div className="c-row c-caption">
              <span className="c-strong">{strength.label}</span>
              <span className="c-mono c-strong">{strength.percent}%</span>
            </div>
            <div className={`c-bar c-bar-thin${position === 2 ? ' c-bar-secondary' : ''}`}>
              <div style={{ width: `${strength.percent}%` }} />
            </div>
          </div>
        ))
      )}
      <button type="button" className="c-btn c-btn-outline c-btn-block c-no-print" onClick={downloadDossier}>
        <Icon name="description" size={18} /> Download Dossier (PDF)
      </button>
    </section>
  );
}
