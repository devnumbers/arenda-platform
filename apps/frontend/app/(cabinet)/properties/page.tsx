import type { Metadata } from 'next';
import { Suspense } from 'react';
import { PropertiesLoading } from '@/widgets/properties/ui/PropertiesLoading';
import { PropertiesPage } from '@/widgets/properties/ui/PropertiesPage';

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
