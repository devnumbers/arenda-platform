import type { JSX } from 'react';
import {
  HubCollapseAnchor,
  HubTitle,
  PageContent,
  TopNav,
} from '@/shared/ui/design';
import { NotificationSettingsSkeleton } from '@/widgets/profile';

/** Route-loading «Уведомлений» (#609): хаб-анатомия страницы (#566) — вне
 * фазы загрузки, настройки — скелетоном (§7). */
export default function NotificationsLoading(): JSX.Element {
  return (
    <>
      <TopNav mobileWings collapse={{ title: 'Уведомления' }} />
      <PageContent>
        <HubCollapseAnchor>
          <HubTitle>Уведомления</HubTitle>
        </HubCollapseAnchor>
        <NotificationSettingsSkeleton />
      </PageContent>
    </>
  );
}
