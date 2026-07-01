import type { Metadata } from 'next';
import { Suspense } from 'react';
import { PageHeader } from '@/shared/ui/page-header';
import { ROUTES } from '@/shared/config/routes';
import { PropertiesPage, PropertiesLoading } from '@/widgets/properties';

export const metadata: Metadata = {
  title: 'Архивные объекты — Рентли',
  description: 'Архивные объекты',
};

export default function PropertiesArchivePage() {
  return (
    <>
      <PageHeader title="Архивные объекты" backHref={ROUTES.properties} />
      <Suspense fallback={<PropertiesLoading />}>
        <PropertiesPage mode="archived" />
      </Suspense>
    </>
  );
}
