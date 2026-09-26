import type { Metadata } from 'next';
import { OperationDetailScreen } from '@/widgets/payments';

/**
 * Страница операции: вхождение правила с «Отметить оплаченной» (и экраном
 * успеха «Платеж оплачен») по макетам 1386:67731 / 1419:25859 /
 * 1419:25645 / 1444:65733. Оболочка новых экранов — из layout группы (screens).
 */

export const metadata: Metadata = {
  title: 'Операция — Рентли',
};

export default async function OperationRoutePage({ params }: PageProps<'/properties/[id]/operations/[operationId]'>) {
  const { id, operationId } = await params;

  return <OperationDetailScreen propertyId={id} operationId={operationId} />;
}
