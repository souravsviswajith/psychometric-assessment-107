/**
 * app/icons.tsx — Material Symbols (outlined, weight 400), the icon set of the
 * client design, bundled as SVG so the app works offline.
 */
import type { ReactNode } from 'react';

const svgs = import.meta.glob<string>(
  '../../node_modules/@material-symbols/svg-400/outlined/{menu,account_balance,search,verified,arrow_forward,arrow_back,account_tree,history,chevron_right,home,home-fill,menu_book,menu_book-fill,quiz,quiz-fill,assignment_turned_in,assignment_turned_in-fill,school,school-fill,description,lock,query_stats,architecture,support_agent,groups,construction,palette,hub,check_circle,cancel,contrast,pending,edit,close,info,badge}.svg',
  { query: '?raw', import: 'default', eager: true },
);

const byName = new Map(Object.entries(svgs).map(([path, svg]) => [path.split('/').pop()!.replace('.svg', ''), svg]));

export function Icon(props: { name: string; filled?: boolean; size?: number }): ReactNode {
  const svg = byName.get(props.filled === true ? `${props.name}-fill` : props.name) ?? byName.get(props.name) ?? '';
  return (
    <span
      className="ms"
      aria-hidden="true"
      style={{ width: props.size ?? 20, height: props.size ?? 20 }}
      dangerouslySetInnerHTML={{ __html: svg }}
    />
  );
}
