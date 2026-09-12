import type { JSX } from 'react';
import { ROUTES } from '@/shared/config/routes';
import { SubScreenShell } from '@/shared/ui/design';
import { PaymentDetailSkeleton } from '@/widgets/profile';

/** Route-loading карточки платежа по тарифу (#609): шапка — вне фазы
 * загрузки, карточка — скелетоном (§7). */
export default function TariffPaymentDetailLoading(): JSX.Element {
  return (
    <SubScreenShell title="Платёж" fallbackHref={ROUTES.profilePayments}>
      <PaymentDetailSkeleton />
    </SubScreenShell>
  );
}
