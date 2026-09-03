import { Suspense } from 'react';
import type { Metadata } from 'next';
import { OperationsOfTypeScreen } from '@/widgets/payments';

/**
 * Экран «Расходы объекта» (#475, Figma 1492-59865): оплаченные расходы за
 * период с листанием и H1-суммой; фильтры (#477) живут в query-параметрах —
 * useSearchParams за Suspense-границей (требование App Router).
 */

export const metadata: Metadata = {
  title: 'Расходы объекта — Рентли',
};

type OperationsExpensePageProps = {
  params: Promise<{ id: string }>;
};

export default async function PropertyOperationsExpenseRoutePage({
  params,
}: OperationsExpensePageProps) {
  const { id } = await params;

  return (
    <Suspense fallback={null}>
      <OperationsOfTypeScreen propertyId={id} type="expense" />
    </Suspense>
  );
}
