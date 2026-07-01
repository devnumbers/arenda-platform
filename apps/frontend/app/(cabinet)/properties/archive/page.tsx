import type { Metadata } from 'next';
import { Suspense } from 'react';
import { PageHeader } from '@/shared/ui/page-header';
import { PageShell } from '@/shared/ui/page-shell';
import { ROUTES } from '@/shared/config/routes';
import { PropertiesPage, PropertiesLoading } from '@/widgets/properties';

export const metadata: Metadata = {
  title: 'Архивные объекты — Arenda Platform',
  description: 'Архивные объекты',
};

export default function PropertiesArchivePage() {
  return (
    <PageShell>
      <PageHeader title="Архивные объекты" backHref={ROUTES.properties} />
      <Suspense fallback={<PropertiesLoading />}>
        <PropertiesPage mode="archived" />
      </Suspense>
    </PageShell>
  );
}
