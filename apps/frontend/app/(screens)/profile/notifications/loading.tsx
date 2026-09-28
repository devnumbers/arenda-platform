import type { JSX } from 'react';
import { SubScreenShell } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { NotificationSettingsContentSkeleton } from '@/widgets/notifications';

/** Route-loading «Настроить уведомления» (#609): каркас саб-экрана (#568)
 * рендерит сам лоадинг (в отличие от страницы, где шелл держит page.tsx);
 * контент — общий скелетон экрана (тот же держит экран в pending пробы
 * браузера — аудит #877). */
export default function NotificationsLoading(): JSX.Element {
  return (
    <SubScreenShell title="Настроить уведомления" fallbackHref={ROUTES.profile}>
      <NotificationSettingsContentSkeleton />
    </SubScreenShell>
  );
}

