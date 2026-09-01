import type { Metadata } from 'next';
import { OperationsSearchStubScreen } from './operations-search-stub-screen';

/**
 * Временная заглушка поиска операций (иконка поиска «Операций объекта»,
 * #474). Полноценный экран — тикет #476.
 */

export const metadata: Metadata = {
  title: 'Поиск операций — Рентли',
};

type OperationsSearchPageProps = {
  params: Promise<{ id: string }>;
};

export default async function PropertyOperationsSearchRoutePage({
  params,
}: OperationsSearchPageProps) {
  const { id } = await params;

  return <OperationsSearchStubScreen propertyId={id} />;
}
