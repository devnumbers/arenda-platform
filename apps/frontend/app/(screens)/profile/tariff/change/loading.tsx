import type { JSX } from 'react';
import { ROUTES } from '@/shared/config/routes';
import { SubScreenShell } from '@/shared/ui/design';
import { TariffChangeSkeleton } from '@/widgets/profile';

/** Route-loading формы «Сменить тариф» (#609): шапка — вне фазы загрузки,
 * список тарифов — скелетоном (§7). */
export default function TariffChangeLoading(): JSX.Element {
  return (
    <SubScreenShell title="Сменить тариф" fallbackHref={ROUTES.profileTariff}>
      <TariffChangeSkeleton />
    </SubScreenShell>
  );
}
