import type { Metadata } from 'next';
import { PaymentsCatalogScreen } from '@/widgets/payments';

/**
 * Страница секции «Просроченные операции» (Figma 1043:57610): полный список
 * просроченных операций объекта, клик по заголовку секции главного экрана.
 */

export const metadata: Metadata = {
  title: 'Просроченные операции — Рентли',
};

export default async function PropertyPaymentsOverdueRoutePage({
  params,
}: PageProps<'/properties/[id]/payments/overdue'>) {
  const { id } = await params;

  return <PaymentsCatalogScreen propertyId={id} variant="overdue" />;
}
