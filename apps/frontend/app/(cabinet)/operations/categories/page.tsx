import { Suspense } from 'react';
import type { Metadata } from 'next';
import { OperationsGlobalCategoriesScreen } from '@/widgets/payments';

/**
 * Страница «Выбрать категорию» (#544): список категорий с суммами периода
 * по всем видимым объектам — мультивыбор для фильтра «Категория» глобальной
 * ленты. Черновик и фильтры живут в query-параметрах — useSearchParams за
 * Suspense-границей (требование App Router).
 */

export const metadata: Metadata = {
  title: 'Выбрать категорию — Рентли',
};

export default function OperationsCategoriesRoutePage() {
  return (
    <Suspense fallback={null}>
      <OperationsGlobalCategoriesScreen />
    </Suspense>
  );
}
