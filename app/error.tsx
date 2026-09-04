'use client';

import { AlertTriangle, RefreshCw } from 'lucide-react';

export default function ErrorPage({ reset }: { error: Error & { digest?: string }; reset: () => void }) {
  return <main className="full-state"><span><AlertTriangle size={23} /></span><p className="section-kicker">Something went wrong</p><h1>We couldn’t load this page</h1><p>The workspace is safe. Try loading this section again.</p><button className="primary-button" onClick={reset}><RefreshCw size={14} />Try again</button></main>;
}
