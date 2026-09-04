import type { Metadata } from 'next';
import Dashboard from '../dashboard';

export const metadata: Metadata = {
  title: 'Developer Documentation',
  description: 'Learn to build, install, test and release apps on Emisell App Platform.',
  robots: { index: false, follow: false },
};

export default function DeveloperDocsPage() {
  return <Dashboard />;
}
