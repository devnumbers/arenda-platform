import type { Metadata } from 'next';
import { PropertiesPage, parseSortFromParams } from '@/widgets/properties';

export const metadata: Metadata = {
  title: 'Архивные объекты — Рентли',
  description: 'Архивные объекты',
};

type PropertiesArchivePageProps = {
  readonly searchParams?: Promise<Record<string, string | string[] | undefined>>;
};

export default async function PropertiesArchivePage({ searchParams }: PropertiesArchivePageProps) {
  const resolved = searchParams ? await searchParams : {};
  const initialSort = parseSortFromParams(resolved);

  return <PropertiesPage mode="archived" initialSort={initialSort} />;
}
