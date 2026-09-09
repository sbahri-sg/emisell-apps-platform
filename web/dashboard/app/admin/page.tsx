import type { Metadata } from 'next';
import UnifiedPortal from '@/components/unified-portal';

export const metadata: Metadata = {
  title: 'Admin — Emisell Apps Platform',
  robots: { index: false, follow: false },
};

export default function Admin() {
  return <UnifiedPortal surface="admin" />;
}
