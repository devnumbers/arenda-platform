import type { Metadata } from 'next';
import { Suspense } from 'react';
import { PropertyLeasesPage } from '@/widgets/property-detail';
import { FinanceLoading } from '@/widgets/finance/ui/FinanceLoading';

export const metadata: Metadata = {
  title: 'Аренды объекта — Arenda Platform',
  description: 'История аренд по объекту недвижимости',
};

export default function PropertyLeasesRoutePage() {
  return (
    <Suspense fallback={<FinanceLoading />}>
      <PropertyLeasesPage />
    </Suspense>
  );
}
