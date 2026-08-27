import type { Metadata } from 'next';
import { PaymentHistoryScreen } from '@/widgets/payments';

/**
 * Подэкран «История платежей» (#466): paid-вхождения, группы по датам,
 * чип сортировки «Новые», порции по 50 с бесконечным скроллом.
 */

export const metadata: Metadata = {
  title: 'История платежей — Рентли',
};

type HistoryRoutePageProps = {
  params: Promise<{ id: string; paymentId: string }>;
};

export default async function PaymentHistoryRoutePage({ params }: HistoryRoutePageProps) {
  const { id, paymentId } = await params;

  return <PaymentHistoryScreen propertyId={id} paymentId={paymentId} />;
}
