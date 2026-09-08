import { Suspense } from 'react';
import type { Metadata } from 'next';
import { OperationsGlobalSearchScreen } from '@/widgets/payments';

/**
 * Страница поиска глобальных операций (#543, Figma 1726-90433): шапка
 * «назад» + поле, чипы совпавших категорий и лента операций всех видимых
 * объектов с подзаголовком-объектом. Запрос (?q=) и фильтры ленты живут
 * в query-параметрах — useSearchParams за Suspense-границей (требование
 * App Router).
 */

export const metadata: Metadata = {
  title: 'Поиск операций — Рентли',
};

export default function OperationsGlobalSearchRoutePage() {
  return (
    <Suspense fallback={null}>
      <OperationsGlobalSearchScreen />
    </Suspense>
  );
}
