import type { Metadata } from 'next';
import { PaymentOverdueScreen } from '@/widgets/payments';

/**
 * Полный список просроченных — подэкран страницы платежа (#466): red-
 * стилизация строк, порции по 50 с бесконечным скроллом.
 */

export const metadata: Metadata = {
  title: 'Просроченные операции — Рентли',
};

type OverdueRoutePageProps = {
  params: Promise<{ id: string; paymentId: string }>;
};

export default async function PaymentOverdueRoutePage({ params }: OverdueRoutePageProps) {
  const { id, paymentId } = await params;

  return <PaymentOverdueScreen propertyId={id} paymentId={paymentId} />;
}
