import type { JSX } from 'react';
import { ROUTES } from '@/shared/config/routes';
import { SubScreenShell } from '@/shared/ui/design';
import { PaymentListSkeleton } from '@/widgets/profile';

/** Route-loading «Истории платежей» (#609): шапка — вне фазы загрузки,
 * список — скелетоном (§7). */
export default function TariffPaymentsLoading(): JSX.Element {
  return (
    <SubScreenShell title="История платежей" fallbackHref={ROUTES.profileTariff}>
      <PaymentListSkeleton />
    </SubScreenShell>
  );
}
