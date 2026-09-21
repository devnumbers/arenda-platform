import type { Metadata } from 'next';
import { parseHistoryOrderParams } from '@/features/payments';
import { PaymentHistoryScreen } from '@/widgets/payments';

/**
 * Подэкран «История платежей» (#466): paid-вхождения, группы по датам,
 * чип сортировки «Новые», порции по 50 с бесконечным скроллом. Направление
 * живёт в адресе (?order=asc, #785) — стартовое значение парсится здесь,
 * на сервере.
 */

export const metadata: Metadata = {
  title: 'История операций — Рентли',
};

type HistoryRoutePageProps = {
  params: Promise<{ id: string; paymentId: string }>;
  searchParams?: Promise<Record<string, string | string[] | undefined>>;
};

export default async function PaymentHistoryRoutePage({ params, searchParams }: HistoryRoutePageProps) {
  const { id, paymentId } = await params;
  const resolved = searchParams ? await searchParams : {};
  const initialOrder = parseHistoryOrderParams(resolved.order);

  return <PaymentHistoryScreen propertyId={id} paymentId={paymentId} initialOrder={initialOrder} />;
}
