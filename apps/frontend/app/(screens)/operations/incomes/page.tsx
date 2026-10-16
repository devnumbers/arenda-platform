import { Suspense } from 'react';
import type { Metadata } from 'next';
import { OperationsGlobalDirectionScreen, OperationsDirectionLoading } from '@/widgets/payments';

/**
 * Страница «Доходы» (#548): все доходы выбранного скоупа за период.
 * Фильтры живут в query-параметрах — useSearchParams за Suspense-границей
 * (требование App Router).
 */

export const metadata: Metadata = {
  title: 'Доходы — Рентли',
};

export default function OperationsIncomesRoutePage() {
  return (
    <Suspense fallback={<OperationsDirectionLoading title="Доходы" />}>
      <OperationsGlobalDirectionScreen type="income" />
    </Suspense>
  );
}
