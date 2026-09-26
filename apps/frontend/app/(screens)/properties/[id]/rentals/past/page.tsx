import type { Metadata } from 'next';
import { RentalPastScreen } from '@/widgets/rentals';

/**
 * «Прошлые аренды» (#535): список завершённых аренд объекта.
 */

export const metadata: Metadata = {
  title: 'Прошлые аренды — Рентли',
};

export default async function RentalPastRoutePage({ params }: PageProps<'/properties/[id]/rentals/past'>) {
  const { id } = await params;

  return <RentalPastScreen propertyId={id} />;
}
