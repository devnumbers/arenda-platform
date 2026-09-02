import { Suspense, type JSX } from 'react';
import type { Metadata } from 'next';
import { OperationsSearchScreen } from '@/widgets/payments';

/**
 * Экран поиска операций объекта (#476, Figma 1494-61633…): хедер-поиск с
 * чипами совпавших категорий и списком операций. Ввод живёт в query-параметре
 * `q` (useSearchQueryState), поэтому клиентский экран со useSearchParams
 * стоит за Suspense-границей — требование App Router.
 */

export const metadata: Metadata = {
  title: 'Поиск операций — Рентли',
};

type OperationsSearchPageProps = {
  params: Promise<{ id: string }>;
};

export default async function PropertyOperationsSearchRoutePage({
  params,
}: OperationsSearchPageProps): Promise<JSX.Element> {
  const { id } = await params;

  return (
    <Suspense fallback={null}>
      <OperationsSearchScreen propertyId={id} />
    </Suspense>
  );
}
