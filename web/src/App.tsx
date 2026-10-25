import { useEffect, useState, type ReactNode } from 'react';
import { useCareerSession, type SessionApi } from './state/runtime.ts';
import { TechnicalView } from './technical/TechnicalView.tsx';
import { Battery } from './app/Battery.tsx';
import { DossierCard } from './app/Dossier.tsx';
import { Icon } from './app/icons.tsx';
import { Portal, Registration, Syllabus } from './app/Pages.tsx';
import { PathwayDetail, PathwayList, isStale, latestSet } from './app/Pathways.tsx';
import { batteryOrder, initials, latestAnswers, loadProfile, saveProfile, type Profile } from './app/shared.tsx';
import '@fontsource/source-serif-4/latin-400.css';
import '@fontsource/source-serif-4/latin-400-italic.css';
import '@fontsource/source-serif-4/latin-600.css';
import '@fontsource/source-sans-3/latin-400.css';
import '@fontsource/source-sans-3/latin-600.css';
import '@fontsource/source-sans-3/latin-700.css';
import './app/app.css';

type Tab = 'home' | 'syllabus' | 'assess' | 'dossier' | 'portal';

const TABS: readonly { key: Tab; label: string; pill: string; icon: string }[] = [
  { key: 'home', label: 'Home', pill: 'Home Overview', icon: 'home' },
  { key: 'syllabus', label: 'Syllabus', pill: 'Assessment Scope', icon: 'menu_book' },
  { key: 'assess', label: 'Assess', pill: 'Live Test', icon: 'quiz' },
  { key: 'dossier', label: 'Dossier', pill: 'Results Dossier', icon: 'assignment_turned_in' },
  { key: 'portal', label: 'Portal', pill: 'Student Portal', icon: 'school' },
];

export function App(): ReactNode {
  const session = useCareerSession();
  if (session.status === 'ready') return <MobileApp api={session.api} />;
  return (
    <div className="c-app">
      <div className="c-screen c-center">
        {session.status === 'loading' ? (
          <div className="c-spinner" aria-label="Loading" />
        ) : (
          <section className="c-card">
            <h1 className="c-h2">The assessment engine did not respond</h1>
            <p className="c-body-sm">{session.error}</p>
            <button type="button" className="c-btn c-btn-primary" onClick={session.retry}>
              Try Again
            </button>
          </section>
        )}
      </div>
    </div>
  );
}

function MobileApp({ api }: { api: SessionApi }): ReactNode {
  const items = batteryOrder(api.catalog.items);
  const firstOpen = (): number => {
    const index = items.findIndex((item) => !latestAnswers(api.view).has(item.itemRef));
    return index < 0 ? items.length : index;
  };
  const [profile, setProfile] = useState<Profile | undefined>(loadProfile);
  const [tab, setTab] = useState<Tab>('home');
  const [cursor, setCursor] = useState(firstOpen);
  const [pathwayId, setPathwayId] = useState<string | undefined>(undefined);
  const [technical, setTechnical] = useState(false);
  const [query, setQuery] = useState<string | undefined>(undefined);

  useEffect(() => {
    window.scrollTo(0, 0);
  }, [tab, pathwayId, technical]);

  // Keep the dossier mapped from the latest answer to each question.
  useEffect(() => {
    if (!api.busy && api.rejections.length === 0 && isStale(api)) void api.mapPathways();
  }, [api]);

  // Android's Back button asks the page first (see MainActivity).
  useEffect(() => {
    const w = window as unknown as { __career107Back?: () => boolean };
    w.__career107Back = () => {
      if (technical) setTechnical(false);
      else if (query !== undefined) setQuery(undefined);
      else if (pathwayId !== undefined) setPathwayId(undefined);
      else if (tab !== 'home') setTab('home');
      else return false;
      return true;
    };
    return () => {
      delete w.__career107Back;
    };
  }, [technical, query, pathwayId, tab]);

  if (technical) return <TechnicalView api={api} onClose={() => setTechnical(false)} />;

  const updateProfile = (next: Profile | undefined): void => {
    saveProfile(next);
    setProfile(next);
  };

  if (profile === undefined) {
    return (
      <div className="c-app">
        <Registration questionCount={items.length} onSubmit={updateProfile} />
      </div>
    );
  }

  const go = (next: Tab): void => {
    setPathwayId(undefined);
    setQuery(undefined);
    setTab(next);
  };
  const openPathway = (id: string): void => {
    go('dossier');
    setPathwayId(id);
  };
  const openItem = (index: number): void => {
    setCursor(index);
    go('assess');
  };

  const battery = <Battery api={api} profile={profile} index={cursor} onIndex={setCursor} onDone={() => go('dossier')} />;
  let screen: ReactNode;
  if (query !== undefined) screen = <Search api={api} query={query} onPathway={openPathway} onItem={openItem} />;
  else if (tab === 'syllabus') screen = <Syllabus api={api} onItem={openItem} />;
  else if (tab === 'assess') screen = battery;
  else if (tab === 'dossier' && pathwayId !== undefined) screen = <PathwayDetail api={api} pathwayId={pathwayId} onBack={() => setPathwayId(undefined)} />;
  else if (tab === 'dossier') {
    screen = (
      <>
        <DossierCard api={api} profile={profile} />
        <PathwayList api={api} onOpen={setPathwayId} onStart={() => go('assess')} />
      </>
    );
  } else if (tab === 'portal') {
    screen = (
      <Portal
        api={api}
        profile={profile}
        onProfile={updateProfile}
        onTechnical={() => setTechnical(true)}
        onReset={() => {
          void api.reset().then(() => {
            updateProfile(undefined);
            setCursor(0);
            go('home');
          });
        }}
      />
    );
  } else {
    screen = (
      <>
        <Hero api={api} onStart={() => openItem(firstOpen())} onExplore={() => go('dossier')} />
        {battery}
        <PathwayList api={api} limit={4} onOpen={openPathway} onStart={() => go('assess')} />
        <DossierCard api={api} profile={profile} />
        <p className="c-footnote">
          <Icon name="lock" size={14} /> Stored only on this device · Self-Description Battery v{items[0]?.instrumentVersion} (draft, unvalidated)
        </p>
      </>
    );
  }

  return (
    <div className="c-app has-nav">
      <header className="c-header">
        <div className="c-header-bar">
          <button type="button" className="c-icon-btn" aria-label="Open portal" onClick={() => go('portal')}>
            <Icon name="menu" size={22} />
          </button>
          <div className="c-brand">
            <span className="c-seal">
              <Icon name="account_balance" size={18} />
            </span>
            <div>
              <strong>Career Intelligence</strong>
              <small>International University</small>
            </div>
          </div>
          <button type="button" className="c-icon-btn" aria-label="Search" onClick={() => setQuery(query === undefined ? '' : undefined)}>
            <Icon name={query === undefined ? 'search' : 'close'} />
          </button>
          <button type="button" className="c-avatar c-avatar-sm" aria-label="Student portal" onClick={() => go('portal')}>
            {initials(profile.name)}
          </button>
        </div>
        {query === undefined ? (
          <nav className="c-pills c-pills-bar" aria-label="Sections">
            {TABS.map((entry) => (
              <button key={entry.key} type="button" className={tab === entry.key ? 'is-active' : ''} onClick={() => go(entry.key)}>
                {entry.pill}
              </button>
            ))}
          </nav>
        ) : (
          <div className="c-search">
            <input className="c-input" autoFocus value={query} placeholder="Search pathways and assessment items" onChange={(event) => setQuery(event.target.value)} />
          </div>
        )}
      </header>

      <main className="c-screen c-stack-lg">{screen}</main>

      {api.rejections.length === 0 ? null : (
        <div className="c-toast" role="alert">
          <p>{api.rejections.map((item) => item.replace(/^[A-Z0-9.\-_]+: /, '')).join(' ')}</p>
          <button type="button" className="c-icon-btn" aria-label="Dismiss" onClick={api.clearMessages}>
            <Icon name="close" size={18} />
          </button>
        </div>
      )}

      <nav className="c-nav" aria-label="Main">
        {TABS.map((entry) => (
          <button key={entry.key} type="button" className={tab === entry.key ? 'is-active' : ''} aria-current={tab === entry.key ? 'page' : undefined} onClick={() => go(entry.key)}>
            <Icon name={entry.icon} filled={tab === entry.key} size={22} />
            <span>{entry.label}</span>
          </button>
        ))}
      </nav>
    </div>
  );
}

function Hero(props: { api: SessionApi; onStart: () => void; onExplore: () => void }): ReactNode {
  const { api } = props;
  const items = api.catalog.items;
  const answered = latestAnswers(api.view).size;
  const benchmarks: readonly [string, string][] = [
    [`${answered}/${items.length}`, 'Items Recorded'],
    [String(latestSet(api)?.pathways.length ?? '—'), 'Pathways Mapped'],
    [String(new Set(items.map((item) => item.constructs[0]?.constructId)).size), 'Dimensions'],
    [String(api.view.decisions.length), 'Decisions Logged'],
  ];
  return (
    <section className="c-card">
      <span className="c-tag">
        <Icon name="verified" size={14} /> Directorate of Career Intelligence
      </span>
      <h1 className="c-h1">Discover Where Your Potential Can Take You</h1>
      <p className="c-body-sm">Explore strengths, working preferences, and career pathways through a structured self-description assessment.</p>
      <div className="c-stack">
        <button type="button" className="c-btn c-btn-primary c-btn-block" onClick={props.onStart}>
          {answered === 0 ? 'Start Career Assessment' : answered === items.length ? 'Review Assessment' : 'Resume Career Assessment'} <small>(about 5 min)</small>
          <Icon name="arrow_forward" size={18} />
        </button>
        <button type="button" className="c-btn c-btn-secondary c-btn-block" onClick={props.onExplore}>
          <Icon name="account_tree" size={18} /> Explore Pathways
        </button>
      </div>
      <div className="c-grid-2 c-benchmarks">
        {benchmarks.map(([value, label]) => (
          <div key={label} className="c-stat">
            <strong>{value}</strong>
            <span>{label}</span>
          </div>
        ))}
      </div>
    </section>
  );
}

function Search(props: { api: SessionApi; query: string; onPathway: (id: string) => void; onItem: (index: number) => void }): ReactNode {
  const needle = props.query.trim().toLowerCase();
  const items = batteryOrder(props.api.catalog.items);
  const pathways = (latestSet(props.api)?.pathways ?? []).filter((pathway) => `${pathway.name} ${pathway.description}`.toLowerCase().includes(needle));
  const matches = items.map((item, index) => ({ item, index })).filter(({ item }) => item.promptText.toLowerCase().includes(needle));
  if (needle === '') return <p className="c-body-sm">Type to search career pathways and assessment items.</p>;
  return (
    <section className="c-card">
      <h2 className="c-h2">Search Results</h2>
      <div className="c-rule" />
      <div className="c-ledger">
        {pathways.map((pathway) => (
          <button key={pathway.id} type="button" onClick={() => props.onPathway(pathway.pathwayId)}>
            <Icon name="account_tree" size={18} />
            <span className="c-body-sm c-strong">{pathway.name}</span>
            <Icon name="chevron_right" size={18} />
          </button>
        ))}
        {matches.map(({ item, index }) => (
          <button key={item.itemRef} type="button" onClick={() => props.onItem(index)}>
            <Icon name="quiz" size={18} />
            <span className="c-body-sm">{item.promptText}</span>
            <Icon name="chevron_right" size={18} />
          </button>
        ))}
        {pathways.length + matches.length === 0 ? <p className="c-body-sm">Nothing matches “{props.query}”.</p> : null}
      </div>
    </section>
  );
}
