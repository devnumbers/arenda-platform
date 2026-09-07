import type { Metadata } from 'next';
import { RentalCompleteScreen } from '@/widgets/rentals';

/**
 * «Завершение аренды» (#534): мастер из подтверждения, даты окончания,
 * возврата залога и итогов; финал — «Аренда объекта … завершена».
 */

export const metadata: Metadata = {
  title: 'Завершение аренды — Рентли',
};

type RentalCompleteRoutePageProps = {
  params: Promise<{ id: string }>;
};

export default async function RentalCompleteRoutePage({
  params,
}: RentalCompleteRoutePageProps) {
  const { id } = await params;

  return <RentalCompleteScreen propertyId={id} />;
}
