import type { JSX } from 'react';
import {
  HubCollapseAnchor,
  HubTitle,
  PageContent,
  TopNav,
} from '@/shared/ui/design';

/** Route-loading «Уведомлений» (#609): хаб-анатомия страницы (#566) — вне
 * фазы загрузки. Контента под загрузкой нет — страница-заглушка карты
 * #734 (см. page.tsx). */
export default function NotificationsLoading(): JSX.Element {
  return (
    <>
      <TopNav mobileWings collapse={{ title: 'Уведомления' }} />
      <PageContent>
        <HubCollapseAnchor>
          <HubTitle>Уведомления</HubTitle>
        </HubCollapseAnchor>
      </PageContent>
    </>
  );
}
