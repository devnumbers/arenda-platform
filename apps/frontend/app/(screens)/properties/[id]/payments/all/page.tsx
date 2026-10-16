import type { Metadata } from 'next';
import { PaymentsCatalogScreen } from '@/widgets/payments';

/**
 * Страница секции «Платежи» (Figma 1043:60174): полный список ручных правил
 * объекта в порядке ближайшего вхождения, включая паузные; автоплатежи
 * живут на соседнем /auto (#1071).
 */

export const metadata: Metadata = {
  title: 'Платежи объекта — Рентли',
};

export default async function PropertyPaymentsAllRoutePage({ params }: PageProps<'/properties/[id]/payments/all'>) {
  const { id } = await params;

  return <PaymentsCatalogScreen propertyId={id} variant="payments" />;
}
