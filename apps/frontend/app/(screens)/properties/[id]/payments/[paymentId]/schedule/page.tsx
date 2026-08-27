import type { Metadata } from 'next';
import { PaymentScheduleScreen } from '@/widgets/payments';

/**
 * Подэкран «График платежей» (#466): материализованное ближайшее плановое
 * вхождение и клиентская проекция следующих, порции по 50 до endDate.
 */

export const metadata: Metadata = {
  title: 'График платежей — Рентли',
};

type ScheduleRoutePageProps = {
  params: Promise<{ id: string; paymentId: string }>;
};

export default async function PaymentScheduleRoutePage({ params }: ScheduleRoutePageProps) {
  const { id, paymentId } = await params;

  return <PaymentScheduleScreen propertyId={id} paymentId={paymentId} />;
}
