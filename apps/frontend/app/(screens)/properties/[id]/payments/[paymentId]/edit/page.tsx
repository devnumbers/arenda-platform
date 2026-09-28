import type { Metadata } from 'next';
import { PaymentEditScreen } from '@/widgets/payments';

/**
 * Экран правки платежа (#467): форма, не визард; сохранение — PATCH,
 * удаление — модалка выбора судьбы просрочек (только владелец).
 */

export const metadata: Metadata = {
  title: 'Редактирование платежа — Рентли',
};

export default async function PaymentEditRoutePage({ params }: PageProps<'/properties/[id]/payments/[paymentId]/edit'>) {
  const { id, paymentId } = await params;

  return <PaymentEditScreen propertyId={id} paymentId={paymentId} />;
}
