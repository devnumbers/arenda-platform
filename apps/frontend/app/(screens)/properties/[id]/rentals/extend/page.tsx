import type { Metadata } from 'next';
import { RentalExtendScreen } from '@/widgets/rentals';

/**
 * «Продление аренды» (#533): новая дата окончания текущей аренды
 * («строго позже» — ADR 0053 §5), попап успеха и пересчёт детализации.
 */

export const metadata: Metadata = {
  title: 'Продление аренды — Рентли',
};

export default async function RentalExtendRoutePage({ params }: PageProps<'/properties/[id]/rentals/extend'>) {
  const { id } = await params;

  return <RentalExtendScreen propertyId={id} />;
}
