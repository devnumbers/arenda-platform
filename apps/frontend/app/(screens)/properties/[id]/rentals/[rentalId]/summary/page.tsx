import type { Metadata } from 'next';
import { RentalSummaryScreen } from '@/widgets/rentals';

/**
 * «Итоги аренды» завершённой (#535): read-only повтор шага итогов мастера
 * (#534) на записанных данных.
 */

export const metadata: Metadata = {
  title: 'Итоги аренды — Рентли',
};

type RentalSummaryRoutePageProps = {
  params: Promise<{ id: string; rentalId: string }>;
};

export default async function RentalSummaryRoutePage({
  params,
}: RentalSummaryRoutePageProps) {
  const { id, rentalId } = await params;

  return <RentalSummaryScreen propertyId={id} rentalId={rentalId} />;
}
