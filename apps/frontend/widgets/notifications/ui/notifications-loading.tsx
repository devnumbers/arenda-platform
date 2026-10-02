import type { JSX } from 'react';
import { Cancel, TrashBin } from '@/shared/assets/icons';
import { IconButton, PageContent, TopNav, TopNavTitle } from '@/shared/ui/design';
import { NotificationDetailSkeleton, NotificationsFeedSkeleton } from './notifications-states';

/** Route-loading архетип зоны уведомлений (#609): шапка — анатомия
 * подэкрана без «Назад» (решение владельца 02.10): крылья только на ПК,
 * тайтл в баре на всех ярусах, кадр совпадает с фазой загрузки экрана,
 * контент — скелетон групп ленты (§7). Используется как loading.tsx
 * сегмента. */
export function NotificationsLoading(): JSX.Element {
  return (
    <>
      <TopNav hideWingsBelowDesktop>
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
