import type { Metadata } from 'next';
import { PaymentHistoryScreen } from '@/widgets/payments';

/**
 * Подэкран «История платежа» (#466, режимы — #1195): дефолт — история
 * С изменениями (чипы журнала вперемешку с операциями по реальному
 * времени); ?changes=0 скрывает изменения — остаются операции по дням.
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
  const initialChangesMode = resolved.changes !== '0';

  return (
    <PaymentHistoryScreen
      propertyId={id}
      paymentId={paymentId}
      initialChangesMode={initialChangesMode}
    />
  );
}
