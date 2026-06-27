import type { Metadata } from 'next';
import { Suspense } from 'react';
import { PropertyOperationsPage } from '@/widgets/property-detail';
import { FinanceLoading } from '@/widgets/finance/ui/FinanceLoading';

export const metadata: Metadata = {
  title: 'Финансы объекта — Arenda Platform',
  description: 'Финансовые операции по объекту недвижимости',
};

export default function PropertyOperationsRoutePage() {
  return (
    <Suspense fallback={<FinanceLoading />}>
      <PropertyOperationsPage />
    </Suspense>
  );
}
