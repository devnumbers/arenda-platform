import type { Metadata } from 'next';
import { PaymentHistoryScreen } from '@/widgets/payments';

/**
 * Подэкран «История платежа» (#466, режимы — #1195): дефолт — операции
 * по дням; ?changes=1 включает режим изменений (журнал правок, ADR 0065).
 * Режим живёт в адресе — стартовое значение парсится здесь, на сервере.
 */

export const metadata: Metadata = {
  title: 'История платежа — Рентли',
};

export default async function PaymentHistoryRoutePage({
  params,
  searchParams,
}: PageProps<'/properties/[id]/payments/[paymentId]/history'>) {
  const { id, paymentId } = await params;
  const resolved = await searchParams;
  const initialChangesMode = resolved.changes === '1';

  return (
    <PaymentHistoryScreen
      propertyId={id}
      paymentId={paymentId}
      initialChangesMode={initialChangesMode}
    />
  );
}
