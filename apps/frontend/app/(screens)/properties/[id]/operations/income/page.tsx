import type { Metadata } from 'next';
import { OperationsOfTypeScreen } from '@/widgets/payments';

/**
 * Экран «Доходы объекта» (#475, Figma 1494-61191): оплаченные доходы за
 * месяц с листанием и H1-суммой периода.
 */

export const metadata: Metadata = {
  title: 'Доходы объекта — Рентли',
};

type OperationsIncomePageProps = {
  params: Promise<{ id: string }>;
};

export default async function PropertyOperationsIncomeRoutePage({
  params,
}: OperationsIncomePageProps) {
  const { id } = await params;

  return <OperationsOfTypeScreen propertyId={id} type="income" />;
}
