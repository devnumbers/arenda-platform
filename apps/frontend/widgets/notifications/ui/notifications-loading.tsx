import type { JSX } from 'react';
import { Cancel, TrashBin } from '@/shared/assets/icons';
import {
  HubCollapseAnchor,
  HubTitle,
  IconButton,
  PageContent,
  TopNav,
} from '@/shared/ui/design';
import { NotificationDetailSkeleton, NotificationsFeedSkeleton } from './notifications-states';

/** Route-loading архетип зоны уведомлений (#609): шапка — канон хаба по
 * макетам 3178 (#1170, карта #1162): крылья и на мобайле, HubTitle
 * в контенте без слота действий (кебаб зависит от данных — §7, без
 * мельканья), контент — скелетон групп ленты. Используется как loading.tsx
 * сегмента. */
export function NotificationsLoading(): JSX.Element {
  return (
    <>
      <TopNav mobileWings collapse={{ title: 'Уведомления' }} />
      <PageContent>
        <HubCollapseAnchor>
          <div className="flex h-8 items-center pr-3.5">
            <HubTitle>Уведомления</HubTitle>
          </div>
        </HubCollapseAnchor>
        <NotificationsFeedSkeleton />
      </PageContent>
    </>
  );
}

/** Route-loading страницы уведомления (#745): бар с постоянными X-закрыть
 * и корзиной (без onClick — фаза загрузки не действует), контент —
 * скелетон детали. */
export function NotificationDetailLoading(): JSX.Element {
  return (
    <>
      <TopNav
        leading={<IconButton icon={<Cancel />} label="Закрыть" />}
        trailing={<IconButton icon={<TrashBin />} label="Удалить" />}
      />
      <PageContent>
        <NotificationDetailSkeleton />
      </PageContent>
    </>
  );
}
