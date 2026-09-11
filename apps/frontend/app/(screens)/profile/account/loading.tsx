import type { JSX } from 'react';
import { ROUTES } from '@/shared/config/routes';
import { SubScreenShell } from '@/shared/ui/design';
import { AccountOverviewSkeleton } from '@/widgets/profile';

/** Route-loading «Аккаунта» (#609): шапка — вне фазы загрузки, обзор —
 * скелетоном (§7). */
export default function AccountLoading(): JSX.Element {
  return (
    <SubScreenShell title="Аккаунт" fallbackHref={ROUTES.profile}>
      <AccountOverviewSkeleton />
    </SubScreenShell>
  );
}
