import type { Metadata } from 'next';
import UnifiedPortal from '@/components/unified-portal';

export const metadata: Metadata = {
  title: 'Developer dashboard — Emisell',
  robots: { index: false, follow: false },
};

export default function Development() {
  return <UnifiedPortal surface="developer" />;
}
