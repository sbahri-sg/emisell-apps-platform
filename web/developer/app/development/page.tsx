import type { Metadata } from 'next';
import Portal from '@/components/portal';

export const metadata: Metadata = {
  title: 'Developer dashboard — Emisell',
  robots: { index: false, follow: false },
};

// Dedicated local entrypoint retains its existing developer-only API proxy.
export default function Development() {
  return <Portal surface="developer" />;
}
