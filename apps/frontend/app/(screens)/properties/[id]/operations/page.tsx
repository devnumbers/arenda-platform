import { Suspense } from 'react';
import type { Metadata } from 'next';
import { OperationsOfPropertyScreen } from '@/widgets/payments';

/**
 * Экран «Операции объекта» (#474, Figma 1492-41825): оплаченные операции
 * за период с чипами фильтров и карточками сводки. Фильтры период/категории
 * (#477) живут в query-параметрах, поэтому клиентский экран со
 * useSearchParams стоит за Suspense-границей — требование App Router.
 */

export const metadata: Metadata = {
  title: 'Операции объекта — Рентли',
};

export default async function PropertyOperationsRoutePage({ params }: PageProps<'/properties/[id]/operations'>) {
  const { id } = await params;

  return (
    <Suspense fallback={null}>
      <OperationsOfPropertyScreen propertyId={id} />
    </Suspense>
  );
}
