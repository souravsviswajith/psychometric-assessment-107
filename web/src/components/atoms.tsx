import type { ReactNode } from 'react';

export function Panel(props: {
  title: string;
  subtitle?: string;
  children: ReactNode;
  className?: string;
  tight?: boolean;
}): ReactNode {
  return (
    <section className={`panel${props.tight === true ? ' tight' : ''}${props.className === undefined ? '' : ` ${props.className}`}`}>
      <header>
        <h2>{props.title}</h2>
        {props.subtitle === undefined ? null : <span className="panel-note mono">{props.subtitle}</span>}
      </header>
      {props.children}
    </section>
  );
}

export function Stat(props: { label: string; value: ReactNode; hint?: string }): ReactNode {
  return (
    <div className="stat">
      <span className="value">{props.value}</span>
      <span className="label">{props.label}</span>
      {props.hint === undefined ? null : <span className="panel-note">{props.hint}</span>}
    </div>
  );
}

export function KV(props: { rows: readonly (readonly [string, ReactNode])[] }): ReactNode {
  return (
    <dl className="kv">
      {props.rows.map(([key, value]) => (
        <div key={key} style={{ display: 'contents' }}>
          <dt>{key}</dt>
          <dd>{value}</dd>
        </div>
      ))}
    </dl>
  );
}

export function Badge(props: { children: ReactNode; tone?: string; title?: string }): ReactNode {
  return (
    <span className={`badge${props.tone === undefined ? '' : ` ${props.tone}`}`} title={props.title}>
      {props.children}
    </span>
  );
}

export function Notice(props: { children: ReactNode; tone?: 'warn' | 'alert' | 'signal' }): ReactNode {
  return <div className={`notice${props.tone === undefined ? '' : ` ${props.tone}`}`}>{props.children}</div>;
}

export function Rejections(props: { items: readonly string[] }): ReactNode {
  if (props.items.length === 0) return null;
  return (
    <Notice tone="alert">
      <strong>Rejected at the boundary.</strong> The system refused these representations before transforming them (§10):
      <ul className="dense">
        {props.items.map((item) => (
          <li key={item} className="rejection">
            {item}
          </li>
        ))}
      </ul>
    </Notice>
  );
}

export function Source(props: { children: ReactNode }): ReactNode {
  return <span className="panel-note mono">{props.children}</span>;
}
