import type { Metadata } from 'next';
import { Suspense } from 'react';
import { PropertiesPage, PropertiesLoading } from '@/widgets/properties';

export const metadata: Metadata = {
  title: 'Архивные объекты — Arenda Platform',
  description: 'Архивные объекты недвижимости',
};

export default function PropertiesArchivePage() {
  return (
    <Suspense fallback={<PropertiesLoading />}>
      <PropertiesPage mode="archived" />
    </Suspense>
  );
}
