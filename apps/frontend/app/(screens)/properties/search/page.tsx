import { Suspense } from 'react';
import type { Metadata } from 'next';
import { PropertiesSearchScreen } from '@/widgets/properties';

/**
 * Страница поиска объектов (#586): поисковая шапка, клиентский фильтр по
 * названию и адресу активной книги. Запрос живёт в query-параметре (?q=)
 * — useSearchParams за Suspense-границей (требование App Router).
 */

export const metadata: Metadata = {
  title: 'Поиск объектов — Рентли',
  description: 'Поиск по объектам',
};

export default function PropertiesSearchRoutePage() {
  return (
    <Suspense fallback={null}>
      <PropertiesSearchScreen />
    </Suspense>
  );
}
