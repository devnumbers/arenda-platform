import { Suspense } from 'react';
import type { Metadata } from 'next';
import { OperationsPeriodScreen } from '@/widgets/payments';

/**
 * Страница «Выберите период» (#477, Figma 1495-64015, 1502-65940,
 * 1502-66758): закреплённый верх (границы + дни недели), бесконечный
 * календарь вверх, закреплённая кнопка «Выбрать». Фильтры живут в
 * query-параметрах — useSearchParams за Suspense-границей (требование
 * App Router).
 */

export const metadata: Metadata = {
  title: 'Выберите период — Рентли',
};

type OperationsPeriodPageProps = {
  params: Promise<{ id: string }>;
};

export default async function PropertyOperationsPeriodRoutePage({
  params,
}: OperationsPeriodPageProps) {
  const { id } = await params;

  return (
    <Suspense fallback={null}>
      <OperationsPeriodScreen propertyId={id} />
    </Suspense>
  );
}
