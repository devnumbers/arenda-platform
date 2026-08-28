import type { Metadata } from 'next';
import { PaymentsCatalogScreen } from '@/widgets/payments';

/**
 * Страница секции «Автоплатежи» (Figma 1043:60502): полный список правил
 * с автоплатежом, включая паузные.
 */

export const metadata: Metadata = {
  title: 'Автоплатежи объекта — Рентли',
};

type AutoCatalogPageProps = {
  params: Promise<{ id: string }>;
};

export default async function PropertyPaymentsAutoRoutePage({ params }: AutoCatalogPageProps) {
  const { id } = await params;

  return <PaymentsCatalogScreen propertyId={id} variant="auto" />;
}
