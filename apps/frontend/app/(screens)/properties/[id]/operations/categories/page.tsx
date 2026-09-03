import { Suspense } from 'react';
import type { Metadata } from 'next';
import { OperationsCategoriesScreen } from '@/widgets/payments';

/**
 * Страница «Выбрать категорию» (#477, Figma 1506-72116, 1510-74149):
 * закреплённый верх (контекстные чипы), строки категорий с суммами периода,
 * закреплённая кнопка «Выбрать». Фильтры живут в query-параметрах —
 * useSearchParams за Suspense-границей (требование App Router).
 */

export const metadata: Metadata = {
  title: 'Выбрать категорию — Рентли',
};

type OperationsCategoriesPageProps = {
  params: Promise<{ id: string }>;
};

export default async function PropertyOperationsCategoriesRoutePage({
  params,
}: OperationsCategoriesPageProps) {
  const { id } = await params;

  return (
    <Suspense fallback={null}>
      <OperationsCategoriesScreen propertyId={id} />
    </Suspense>
  );
}
