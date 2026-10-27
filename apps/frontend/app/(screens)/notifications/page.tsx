import type { Metadata } from 'next';
import { parseStringParam } from '@/shared/lib/parse-string-param';
import { NotificationsFeedScreen } from '@/widgets/notifications';
import {
  notificationsFeedQueryOptions,
  unreadNotificationsCountQueryOptions,
} from '@/features/notifications';
import { ServerPrefetchBoundary } from '@/shared/api/server-prefetch';
import { serverApiClient } from '@/shared/api/server-client';

// Кабинетная страница: куки-сессия и URL-состояние — всегда динамический
// рендер; force-dynamic снимает требование Suspense-границы для
// useSearchParams при статическом экспорте (аменда #1605 — in-page
// Suspense убран ради однофлушного HTML, react/react#37014).
export const dynamic = 'force-dynamic';

/** Центр уведомлений — лента (карта #734, тикет #744). На едином хроме:
 * оболочка — ScreenLayout группы (screens), вход — средний таб TabBar
 * мобайла и пилюля «Уведомления» ПК. Фильтр «Непрочитанные» живёт в query
 * строки (?unread=1) — переживает перезагрузку. Чтение — канон
 * parseStringParam: битый дубликат массивом (?unread=1&unread=1) —
 * дефолт «все», не «первый элемент» (политика parse-enum-param;
 * выравнивание страницы, пришедшей с влитием notifications, — находка К3
 * повторного pre-merge #785, хвост #791). */
export const metadata: Metadata = {
  title: 'Уведомления — Рентли',
  description: 'Центр уведомлений',
};

export default async function NotificationsRoutePage({
  searchParams,
}: PageProps<'/notifications'>) {
  const { unread } = await searchParams;
  const initialUnreadOnly = parseStringParam(unread) === '1';

  // Дефолтный срез ленты — «все»; фильтр «Непрочитанные» живёт в query
  // строки, срез с ним клиент перечитает (канон #887). Счётчик — бейдж
  // пилюли и хаба, в первом кадре всегда.
  return (
    <ServerPrefetchBoundary
      prefetch={(queryClient) => {
        void queryClient.prefetchInfiniteQuery(notificationsFeedQueryOptions({
          unreadOnly: false,
          transport: serverApiClient,
        }));
        void queryClient.prefetchQuery(unreadNotificationsCountQueryOptions(serverApiClient));
      }}
    >
      <NotificationsFeedScreen initialUnreadOnly={initialUnreadOnly} />
    </ServerPrefetchBoundary>
  );
}
