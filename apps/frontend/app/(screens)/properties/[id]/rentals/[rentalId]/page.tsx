import type { Metadata } from 'next';
import { RentalCompletedScreen } from '@/widgets/rentals';

/**
 * Завершённая детализация «Прошлых аренд» (#535): карточка прошлой аренды
 * по id. Незавершённая по прямой ссылке уводится на текущий экран аренды.
 */

export const metadata: Metadata = {
  title: 'Аренда — Рентли',
};

export default async function RentalCompletedRoutePage({
  params,
}: PageProps<'/properties/[id]/rentals/[rentalId]'>) {
  const { id, rentalId } = await params;

  return <RentalCompletedScreen propertyId={id} rentalId={rentalId} />;
}
