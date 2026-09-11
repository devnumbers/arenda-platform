import type { JSX } from 'react';
import { ROUTES } from '@/shared/config/routes';
import { SubScreenShell } from '@/shared/ui/design';
import { PaymentMethodListSkeleton } from '@/widgets/profile';

/** Route-loading «Способов оплаты» (#609): шапка — вне фазы загрузки,
 * список карт — скелетоном (§7). */
export default function PaymentMethodsLoading(): JSX.Element {
  return (
    <SubScreenShell title="Способы оплаты" fallbackHref={ROUTES.profileTariff}>
      <PaymentMethodListSkeleton />
    </SubScreenShell>
  );
}
