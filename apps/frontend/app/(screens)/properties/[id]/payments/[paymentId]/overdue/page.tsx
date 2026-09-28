import type { Metadata } from 'next';
import { PaymentOverdueScreen } from '@/widgets/payments';

/**
 * Полный список просроченных — подэкран страницы платежа (#466): red-
 * стилизация строк, порции по 50 с бесконечным скроллом.
 */

export const metadata: Metadata = {
  title: 'Просроченные операции — Рентли',
};

export default async function PaymentOverdueRoutePage({ params }: PageProps<'/properties/[id]/payments/[paymentId]/overdue'>) {
  const { id, paymentId } = await params;

  return <PaymentOverdueScreen propertyId={id} paymentId={paymentId} />;
}
