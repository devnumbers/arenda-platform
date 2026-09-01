import type { Metadata } from 'next';
import { OperationsOfTypeScreen } from '@/widgets/payments';

/**
 * Экран «Расходы объекта» (#475, Figma 1492-59865): оплаченные расходы за
 * месяц с листанием и H1-суммой периода.
 */

export const metadata: Metadata = {
  title: 'Расходы объекта — Рентли',
};

type OperationsExpensePageProps = {
  params: Promise<{ id: string }>;
};

export default async function PropertyOperationsExpenseRoutePage({
  params,
}: OperationsExpensePageProps) {
  const { id } = await params;

  return <OperationsOfTypeScreen propertyId={id} type="expense" />;
}
