import type { Metadata } from 'next';
import { PaymentDetailScreen } from '@/widgets/payments';

/**
 * Страница платежа (#465): карточка правила, мутации паузы/оплаты/избранного,
 * секции «Ближайший платеж» и «Просроченные», плитки подэкранов. Оболочка
 * новых экранов — из layout группы (screens).
 */

export const metadata: Metadata = {
  title: 'Платеж — Рентли',
};

export default async function PaymentRoutePage({ params }: PageProps<'/properties/[id]/payments/[paymentId]'>) {
  const { id, paymentId } = await params;

  return <PaymentDetailScreen propertyId={id} paymentId={paymentId} />;
}
