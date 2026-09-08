import { Suspense } from 'react';
import type { Metadata } from 'next';
import { OperationsGlobalScreen } from '@/widgets/payments';

/** Экран «Операции» — глобальная лента по всем объектам (карта #545,
 * тикет #541); на едином хроме экранов (#564), вход — «Операции» в
 * сайдбаре ПК и в шите «Еще» на мобайле и планшете. Фильтры — в
 * query-параметрах, поэтому клиентский экран со useSearchParams стоит за
 * Suspense-границей — требование App Router (иначе прод-сборка падает на
 * пререндере). */
export const metadata: Metadata = {
  title: 'Операции — Рентли',
};

export default function OperationsRoutePage() {
  return (
    <Suspense fallback={null}>
      <OperationsGlobalScreen />
    </Suspense>
  );
}
