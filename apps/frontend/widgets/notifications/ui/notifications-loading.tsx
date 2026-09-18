import type { JSX } from 'react';
import { Cancel, TrashBin } from '@/shared/assets/icons';
import { IconButton, PageContent, TopNav, TopNavTitle } from '@/shared/ui/design';
import { NotificationDetailSkeleton, NotificationsFeedSkeleton } from './notifications-states';

/** Route-loading архетип зоны уведомлений (#609): хаб-шапка с заголовком
 * в баре (трейлинг-действия зависят от данных и в фазе загрузки не
 * рисуются), контент — скелетон групп ленты (§7). Используется как
 * loading.tsx сегмента. */
export function NotificationsLoading(): JSX.Element {
  return (
    <>
      <TopNav mobileWings>
        <TopNavTitle title="Уведомления" />
      </TopNav>
      <PageContent>
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
