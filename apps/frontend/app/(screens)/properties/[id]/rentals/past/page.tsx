import type { Metadata } from 'next';
import { RentalPastScreen } from '@/widgets/rentals';

/**
 * «Прошлые аренды» (#535): список завершённых аренд объекта.
 */

export const metadata: Metadata = {
  title: 'Прошлые аренды — Рентли',
};

type RentalPastRoutePageProps = {
  params: Promise<{ id: string }>;
};

export default async function RentalPastRoutePage({ params }: RentalPastRoutePageProps) {
  const { id } = await params;

  return <RentalPastScreen propertyId={id} />;
}
