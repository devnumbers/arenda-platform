import type { Metadata } from 'next';
import { Suspense } from 'react';
import { PropertiesPage } from '@/widgets/properties/ui/PropertiesPage';
import { PropertiesLoading } from '@/widgets/properties/ui/PropertiesLoading';

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
