import type { JSX } from 'react';
import { ROUTES } from '@/shared/config/routes';
import { SubScreenShell } from '@/shared/ui/design';
import { TariffOverviewSkeleton } from '@/widgets/profile';

/** Route-loading «Тарифа» (#609): шапка — вне фазы загрузки, обзор
 * подписки — скелетоном (§7). */
export default function TariffLoading(): JSX.Element {
  return (
    <SubScreenShell title="Тариф" fallbackHref={ROUTES.profile}>
      <TariffOverviewSkeleton />
    </SubScreenShell>
  );
}
