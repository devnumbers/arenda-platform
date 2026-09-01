import type { Metadata } from 'next';
import { OperationsOfPropertyScreen } from '@/widgets/payments';

/**
 * Экран «Операции объекта» (#474, Figma 1492-41825): оплаченные операции
 * за период с чипами фильтров и карточками сводки.
 */

export const metadata: Metadata = {
  title: 'Операции объекта — Рентли',
};

type OperationsPageProps = {
  params: Promise<{ id: string }>;
};

export default async function PropertyOperationsRoutePage({ params }: OperationsPageProps) {
  const { id } = await params;

  return <OperationsOfPropertyScreen propertyId={id} />;
}
