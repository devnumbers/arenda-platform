import type { Metadata } from 'next';
import { Suspense } from 'react';
import { PageHeader } from '@/shared/ui/page-header';
import { PropertiesPage, PropertiesLoading } from '@/widgets/properties';

export const metadata: Metadata = {
  title: 'Мои объекты — Рентли',
  description: 'Список объектов',
};

export default function PropertiesRoutePage() {
  return (
    <>
      <PageHeader title="Мои объекты" />
      <Suspense fallback={<PropertiesLoading />}>
        <PropertiesPage />
      </Suspense>
    </>
  );
}
