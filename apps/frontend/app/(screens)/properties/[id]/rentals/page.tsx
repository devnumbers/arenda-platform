import type { Metadata } from 'next';
import { RentalScreen } from '@/widgets/rentals';

/**
 * Экран «Аренда» (#531): пустое состояние без аренды, детализация текущей.
 */

export const metadata: Metadata = {
  title: 'Аренда — Рентли',
};

export default async function RentalRoutePage({ params }: PageProps<'/properties/[id]/rentals'>) {
  const { id } = await params;

  return <RentalScreen propertyId={id} />;
}
