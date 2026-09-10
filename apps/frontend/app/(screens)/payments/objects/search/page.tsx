import { Suspense } from 'react';
import type { Metadata } from 'next';
import { PaymentsObjectsSearchScreen } from '@/widgets/payments';

/**
 * Страница поиска объектов (карта #573, тикет #582, Figma 888:19356/19364):
 * поисковая шапка «Найти объект» и строки совпавших объектов. Запрос живёт
 * в query-параметре (?q=) — useSearchParams за Suspense-границей (требование
 * App Router).
 */

export const metadata: Metadata = {
  title: 'Поиск объектов — Рентли',
};

export default function PaymentsObjectsSearchRoutePage() {
  return (
    <Suspense fallback={null}>
      <PaymentsObjectsSearchScreen />
    </Suspense>
  );
}
