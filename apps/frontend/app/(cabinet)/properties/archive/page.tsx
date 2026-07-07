import type { Metadata } from 'next';
import { PageHeader } from '@/shared/ui/page-header';
import { ROUTES } from '@/shared/config/routes';
import { PropertiesPage } from '@/widgets/properties';
import { parseFiltersFromParams, parseSortFromParams } from '@/widgets/properties/lib/parse-property-search-params';

export const metadata: Metadata = {
  title: 'Архивные объекты — Рентли',
  description: 'Архивные объекты',
};

type PropertiesArchivePageProps = {
  readonly searchParams?: Promise<Record<string, string | string[] | undefined>>;
};

export default async function PropertiesArchivePage({ searchParams }: PropertiesArchivePageProps) {
  const resolved = searchParams ? await searchParams : {};
  const initialFilters = parseFiltersFromParams(resolved);
  const initialSort = parseSortFromParams(resolved);

  return (
    <>
      <PageHeader title="Архивные объекты" backHref={ROUTES.properties} />
      <PropertiesPage mode="archived" initialFilters={initialFilters} initialSort={initialSort} />
    </>
  );
}
