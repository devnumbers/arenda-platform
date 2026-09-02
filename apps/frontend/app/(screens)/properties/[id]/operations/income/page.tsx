import { Suspense } from 'react';
import type { Metadata } from 'next';
import { OperationsOfTypeScreen } from '@/widgets/payments';

/**
 * Экран «Доходы объекта» (#475, Figma 1494-61191): оплаченные доходы за
 * период с листанием и H1-суммой; фильтры (#477) живут в query-параметрах —
 * useSearchParams за Suspense-границей (требование App Router).
 */

export const metadata: Metadata = {
  title: 'Доходы объекта — Рентли',
};

type OperationsIncomePageProps = {
  params: Promise<{ id: string }>;
};

export default async function PropertyOperationsIncomeRoutePage({
  params,
}: OperationsIncomePageProps) {
  const { id } = await params;

  return (
    <Suspense fallback={null}>
      <OperationsOfTypeScreen propertyId={id} type="income" />
    </Suspense>
  );
}
