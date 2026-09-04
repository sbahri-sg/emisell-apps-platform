import type { Metadata } from 'next';
import AdminDashboard from '../admin-dashboard';

export const metadata: Metadata = {
  title: 'Admin Documentation',
  description: 'Operator guide and verified API contracts for Emisell App Platform.',
  robots: { index: false, follow: false },
};

export default function AdminDocsPage() {
  return <AdminDashboard />;
}
