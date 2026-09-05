import { Suspense } from 'react';
import type { Metadata } from 'next';
import { OperationsObjectsSelectScreen } from '@/widgets/payments';

/**
 * Страница «Выбрать объект» (#542, Figma 1733-26805): мультивыбор объектов
 * для фильтра «Объект» глобальной ленты. Черновик и фильтры живут в
 * query-параметрах — useSearchParams за Suspense-границей (требование
 * App Router).
 */

export const metadata: Metadata = {
  title: 'Выбрать объект — Рентли',
};

export default function OperationsObjectsRoutePage() {
  return (
    <Suspense fallback={null}>
      <OperationsObjectsSelectScreen />
    </Suspense>
  );
}
