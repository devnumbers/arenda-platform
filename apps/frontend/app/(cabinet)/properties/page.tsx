import type { Metadata } from 'next';
import { Suspense } from 'react';
import { PropertiesPage, PropertiesLoading } from '@/widgets/properties';

export const metadata: Metadata = {
  title: 'Мои объекты — Arenda Platform',
  description: 'Список объектов недвижимости',
};

export default function PropertiesRoutePage() {
  return (
    <Suspense fallback={<PropertiesLoading />}>
      <PropertiesPage />
    </Suspense>
  );
}
