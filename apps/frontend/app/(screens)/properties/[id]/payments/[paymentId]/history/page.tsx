import type { Metadata } from 'next';
import { parseHistoryOrderParams } from '@/features/payments';
import { PaymentHistoryScreen } from '@/widgets/payments';

/**
 * Подэкран «История платежей» (#466): paid-вхождения, группы по датам,
 * чип сортировки «Сначала новые», порции по 50 с бесконечным скроллом.
 * Направление
 * живёт в адресе (?order=asc, #785) — стартовое значение парсится здесь,
 * на сервере.
 */

export const metadata: Metadata = {
  title: 'История операций — Рентли',
};

export default async function PaymentHistoryRoutePage({
  params,
  searchParams,
}: PageProps<'/properties/[id]/payments/[paymentId]/history'>) {
  const { id, paymentId } = await params;
  const resolved = await searchParams;
  const initialOrder = parseHistoryOrderParams(resolved.order);

  return <PaymentHistoryScreen propertyId={id} paymentId={paymentId} initialOrder={initialOrder} />;
}
