export default function Loading() {
  return <div className="route-state"><div className="route-state-sidebar" /><main><span className="skeleton-line short" /><span className="skeleton-line title" /><span className="skeleton-line" /><div className="skeleton-grid">{[0, 1, 2, 3].map((item) => <span key={item} />)}</div><span className="skeleton-panel" /></main></div>;
}
