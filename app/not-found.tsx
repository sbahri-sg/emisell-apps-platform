import { ArrowLeft, SearchX } from 'lucide-react';
import Link from 'next/link';

export default function NotFound() {
  return <main className="full-state"><span><SearchX size={23} /></span><p className="section-kicker">404 · Not found</p><h1>This page doesn’t exist</h1><p>The app or workspace page may have moved.</p><Link className="primary-button" href="/apps"><ArrowLeft size={14} />Back to apps</Link></main>;
}
