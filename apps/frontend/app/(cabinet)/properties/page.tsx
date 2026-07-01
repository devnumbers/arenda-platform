import type { Metadata } from 'next';
import { Suspense } from 'react';
import { PageHeader } from '@/shared/ui/page-header';
import { PageShell } from '@/shared/ui/page-shell';
import { PropertiesPage, PropertiesLoading } from '@/widgets/properties';

export const metadata: Metadata = {
  title: 'Мои объекты — Arenda Platform',
  description: 'Список объектов',
};

export default function PropertiesRoutePage() {
  return (
    <PageShell>
      <PageHeader title="Мои объекты" />
      <Suspense fallback={<PropertiesLoading />}>
        <PropertiesPage />
      </Suspense>
    </PageShell>
  );
}
