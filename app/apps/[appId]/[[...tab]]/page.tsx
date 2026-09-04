import type { Metadata } from 'next';
import Dashboard from '../../../dashboard';

export async function generateMetadata({ params }: { params: Promise<{ appId: string }> }): Promise<Metadata> {
  const { appId } = await params;
  const title = appId.split('-').map((part) => part.charAt(0).toUpperCase() + part.slice(1)).join(' ');
  const description = 'App configuration in Emisell App Platform.';
  return {
    title,
    description,
    openGraph: { title, description, images: [] },
    twitter: { title, description, images: [] },
  };
}

export default function AppDetailPage() {
  return <Dashboard />;
}
