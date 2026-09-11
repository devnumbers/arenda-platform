import type { JSX } from 'react';
import { ROUTES } from '@/shared/config/routes';
import { SubScreenShell } from '@/shared/ui/design';
import { ProfileOverviewSkeleton } from '@/widgets/profile';

/**
 * Route-loading профиля (#609): шапка подэкрана — вне фазы загрузки,
 * карточка пользователя — скелетоном; меню и «Выйти» реальными (§7).
 */
export default function ProfileLoading(): JSX.Element {
  return (
    <SubScreenShell title="Профиль" fallbackHref={ROUTES.properties}>
      <ProfileOverviewSkeleton />
    </SubScreenShell>
  );
}
