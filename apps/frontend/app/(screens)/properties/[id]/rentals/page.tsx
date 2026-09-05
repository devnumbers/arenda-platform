import type { Metadata } from 'next';
import { RentalScreen } from '@/widgets/rentals';

/**
 * Экран «Аренда» (#531): пустое состояние без аренды, детализация текущей.
 */

export const metadata: Metadata = {
  title: 'Аренда — Рентли',
};

type RentalRoutePageProps = {
  params: Promise<{ id: string }>;
};

export default async function RentalRoutePage({ params }: RentalRoutePageProps) {
  const { id } = await params;

  return <RentalScreen propertyId={id} />;
}
