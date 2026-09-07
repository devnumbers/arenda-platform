import type { Metadata } from 'next';
import { RentalHistoryScreen } from '@/widgets/rentals';

/**
 * «История операций» завершённой аренды (#535): paid-вхождения её платежа
 * с сортировкой и группировкой по датам.
 */

export const metadata: Metadata = {
  title: 'История операций — Рентли',
};

type RentalHistoryRoutePageProps = {
  params: Promise<{ id: string; rentalId: string }>;
};

export default async function RentalHistoryRoutePage({
  params,
}: RentalHistoryRoutePageProps) {
  const { id, rentalId } = await params;

  return <RentalHistoryScreen propertyId={id} rentalId={rentalId} />;
}
