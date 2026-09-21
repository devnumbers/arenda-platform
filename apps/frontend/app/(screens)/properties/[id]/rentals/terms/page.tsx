import type { Metadata } from 'next';
import { RentalTermsScreen } from '@/widgets/rentals';

/**
 * «Условия аренды» read-only (#531): полный просмотр условий. Без
 * `?rental=` — текущая аренда; с `?rental=<id>` — условия завершённой
 * аренды из «Прошлых аренд» (#535, подзаголовок «В архиве»).
 */

export const metadata: Metadata = {
  title: 'Условия аренды — Рентли',
};

export default async function RentalTermsRoutePage({
  params,
  searchParams,
}: PageProps<'/properties/[id]/rentals/terms'>) {
  const { id } = await params;
  const query = await searchParams;
  const rentalParam = typeof query.rental === 'string' ? query.rental : undefined;

  return <RentalTermsScreen propertyId={id} rentalId={rentalParam} />;
}
