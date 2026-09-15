import { Suspense } from 'react';
import type { Metadata } from 'next';
import { OperationsGlobalDirectionScreen, OperationsDirectionLoading } from '@/widgets/payments';

/**
 * Страница «Расходы» (#548): все расходы выбранного скоупа за период.
 * Фильтры живут в query-параметрах — useSearchParams за Suspense-границей
 * (требование App Router).
 */

export const metadata: Metadata = {
  title: 'Расходы — Рентли',
};

export default function OperationsExpensesRoutePage() {
  return (
    <Suspense fallback={<OperationsDirectionLoading title="Расходы" />}>
      <OperationsGlobalDirectionScreen type="expense" />
    </Suspense>
  );
}
