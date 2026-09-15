import { Suspense } from 'react';
import type { Metadata } from 'next';
import { OperationsOfTypeScreen } from '@/widgets/payments';

/**
 * Экран «Доходы объекта» (#475): оплаченные доходы за период — карточка-
 * сводка и лента по канону глобальных направлений (#679); фильтры (#477)
 * живут в query-параметрах — useSearchParams за Suspense-границей
 * (требование App Router).
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
