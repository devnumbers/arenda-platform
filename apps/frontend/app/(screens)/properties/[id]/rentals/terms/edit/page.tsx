import type { Metadata } from 'next';
import { RentalTermsEditScreen } from '@/widgets/rentals';

/**
 * «Изменить условия» (#532): правка условий текущей аренды, начало
 * read-only (ADR 0053 §3).
 */

export const metadata: Metadata = {
  title: 'Изменить условия — Рентли',
};

type RentalTermsEditRoutePageProps = {
  params: Promise<{ id: string }>;
};

export default async function RentalTermsEditRoutePage({ params }: RentalTermsEditRoutePageProps) {
  const { id } = await params;

  return <RentalTermsEditScreen propertyId={id} />;
}
