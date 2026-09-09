import type { Metadata } from 'next';
import { notFound } from 'next/navigation';
import Documentation from '@/components/documentation';
import { documentationTopics, isDocumentationTopic } from '@/lib/documentation';

export function generateStaticParams() {
  return documentationTopics.map(({ id }) => ({ topic: id }));
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ topic: string }>;
}): Promise<Metadata> {
  const { topic } = await params;
  const entry = documentationTopics.find(({ id }) => id === topic);
  return {
    description: entry?.summary,
    title: entry
      ? `${entry.title} — Emisell Docs`
      : 'Dokumentasi tidak ditemukan',
  };
}

export default async function Guide({
  params,
}: {
  params: Promise<{ topic: string }>;
}) {
  const { topic } = await params;
  if (!isDocumentationTopic(topic)) notFound();
  return <Documentation topic={topic} />;
}
