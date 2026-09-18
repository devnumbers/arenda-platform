import type { JSX } from 'react';
import { PageContent, TopNav, TopNavTitle } from '@/shared/ui/design';
import { NotificationsFeedSkeleton } from './notifications-states';

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
