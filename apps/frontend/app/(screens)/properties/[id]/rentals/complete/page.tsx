import type { Metadata } from 'next';
import { RentalCompleteScreen } from '@/widgets/rentals';

/**
 * «Завершение аренды» (#534): мастер из подтверждения, даты окончания,
 * возврата залога и итогов; финал — «Аренда объекта … завершена».
 */

export const metadata: Metadata = {
  title: 'Завершение аренды — Рентли',
};

export default async function RentalCompleteRoutePage({
  params,
}: PageProps<'/properties/[id]/rentals/complete'>) {
  const { id } = await params;

  return <RentalCompleteScreen propertyId={id} />;
}
