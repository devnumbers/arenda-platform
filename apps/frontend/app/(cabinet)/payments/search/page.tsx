import { Suspense } from 'react';
import type { Metadata } from 'next';
import { PaymentsGlobalSearchScreen } from '@/widgets/payments';

/**
 * Страница поиска платежей (#581, Figma 706:12168/12649/13008, 862:24128,
 * 860:22842): поисковая шапка, чипы совпавших категорий и строки
 * совпавших правил с «Показать все». Запрос живёт в query-параметре (?q=)
 * — useSearchParams за Suspense-границей (требование App Router).
 */

export const metadata: Metadata = {
  title: 'Поиск платежей — Рентли',
};

export default function PaymentsGlobalSearchRoutePage() {
  return (
    <Suspense fallback={null}>
      <PaymentsGlobalSearchScreen />
    </Suspense>
  );
}
