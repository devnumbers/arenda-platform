import type { Metadata } from 'next';
import { NotificationDetailScreen } from '@/widgets/notifications';
import { notificationDetailQueryOptions } from '@/features/notifications';
import { ServerPrefetchBoundary } from '@/shared/api/server-prefetch';
import { serverApiClient } from '@/shared/api/server-client';

// Кабинетная страница: куки-сессия и URL-состояние — всегда динамический
// рендер; force-dynamic снимает требование Suspense-границы для
// useSearchParams при статическом экспорте (аменда #1605 — in-page
// Suspense убран ради однофлушного HTML, react/react#37014).
export const dynamic = 'force-dynamic';

/** Страница уведомления (карта #734, тикет #745): живые действия читателя,
 * прочтение — POST read при открытии, удаление — корзиной в баре. */
export const metadata: Metadata = {
  title: 'Уведомление — Рентли',
  description: 'Страница уведомления',
};

export default async function NotificationRoutePage({ params }: PageProps<'/notifications/[notificationId]'>) {
  const { notificationId } = await params;

  return (
<ServerPrefetchBoundary prefetch={(queryClient) => {
  void queryClient.prefetchQuery(notificationDetailQueryOptions({ id: notificationId, transport: serverApiClient }));
}}>
  <NotificationDetailScreen notificationId={notificationId} />
</ServerPrefetchBoundary>
  );
}
