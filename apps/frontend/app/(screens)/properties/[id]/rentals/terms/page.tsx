import type { Metadata } from 'next';
import { RentalTermsScreen } from '@/widgets/rentals';

/**
 * «Условия аренды» read-only (#531): полный просмотр условий текущей аренды.
 */

export const metadata: Metadata = {
  title: 'Условия аренды — Рентли',
};

type RentalTermsRoutePageProps = {
  params: Promise<{ id: string }>;
};

export default async function RentalTermsRoutePage({ params }: RentalTermsRoutePageProps) {
  const { id } = await params;

  return <RentalTermsScreen propertyId={id} />;
}
