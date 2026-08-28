import type { Metadata } from 'next';
import { PaymentsCatalogScreen } from '@/widgets/payments';

/**
 * Страница секции «Платежи» (Figma 1043:60174): полный список правил
 * объекта в порядке ближайшего вхождения, включая паузные.
 */

export const metadata: Metadata = {
  title: 'Платежи объекта — Рентли',
};

type AllCatalogPageProps = {
  params: Promise<{ id: string }>;
};

export default async function PropertyPaymentsAllRoutePage({ params }: AllCatalogPageProps) {
  const { id } = await params;

  return <PaymentsCatalogScreen propertyId={id} variant="payments" />;
}
