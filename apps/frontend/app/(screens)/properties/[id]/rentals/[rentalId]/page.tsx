import type { Metadata } from 'next';
import { RentalCompletedScreen } from '@/widgets/rentals';

/**
 * Завершённая детализация «Прошлых аренд» (#535): карточка прошлой аренды
 * по id. Незавершённая по прямой ссылке уводится на текущий экран аренды.
 */

export const metadata: Metadata = {
  title: 'Аренда — Рентли',
};

type RentalCompletedRoutePageProps = {
  params: Promise<{ id: string; rentalId: string }>;
};

export default async function RentalCompletedRoutePage({
  params,
}: RentalCompletedRoutePageProps) {
  const { id, rentalId } = await params;

  return <RentalCompletedScreen propertyId={id} rentalId={rentalId} />;
}
