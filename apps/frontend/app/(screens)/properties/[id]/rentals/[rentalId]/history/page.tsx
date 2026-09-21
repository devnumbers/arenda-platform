import type { Metadata } from 'next';
import { parseHistoryOrderParams } from '@/features/payments';
import { RentalHistoryScreen } from '@/widgets/rentals';

/**
 * «История операций» завершённой аренды (#535): paid-вхождения её платежа
 * с сортировкой и группировкой по датам. Направление живёт в адресе
 * (?order=asc, #785) — стартовое значение парсится здесь, на сервере.
 */

export const metadata: Metadata = {
  title: 'История операций — Рентли',
};

export default async function RentalHistoryRoutePage({
  params,
  searchParams,
}: PageProps<'/properties/[id]/rentals/[rentalId]/history'>) {
  const { id, rentalId } = await params;
  const resolved = await searchParams;
  const initialOrder = parseHistoryOrderParams(resolved.order);

  return (
    <RentalHistoryScreen propertyId={id} rentalId={rentalId} initialOrder={initialOrder} />
  );
}
