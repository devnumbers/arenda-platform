import type { Metadata } from 'next';
import { PaymentsCatalogScreen } from '@/widgets/payments';

/**
 * Страница секции «Просроченные операции» (Figma 1043:57610): полный список
 * просроченных операций объекта, клик по заголовку секции главного экрана.
 */

export const metadata: Metadata = {
  title: 'Просроченные операции — Рентли',
};

type OverdueCatalogPageProps = {
  params: Promise<{ id: string }>;
};

export default async function PropertyPaymentsOverdueRoutePage({
  params,
}: OverdueCatalogPageProps) {
  const { id } = await params;

  return <PaymentsCatalogScreen propertyId={id} variant="overdue" />;
}
