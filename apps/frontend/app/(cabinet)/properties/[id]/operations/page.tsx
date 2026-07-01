import type { Metadata } from 'next';
import { Suspense } from 'react';
import { PageShell } from '@/shared/ui/page-shell';
import { PropertyOperationsPage } from '@/widgets/property-detail';
import { FinanceLoading } from '@/widgets/finance/ui/FinanceLoading';

export const metadata: Metadata = {
  title: 'Финансы объекта — Рентли',
  description: 'Финансовые операции по объекту недвижимости',
};

export default function PropertyOperationsRoutePage() {
  return (
    <PageShell>
      <Suspense fallback={<FinanceLoading />}>
        <PropertyOperationsPage />
      </Suspense>
    </PageShell>
  );
}
