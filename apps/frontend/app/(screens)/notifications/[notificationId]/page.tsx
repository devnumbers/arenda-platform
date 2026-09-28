import { Suspense } from 'react';
import type { Metadata } from 'next';
import { NotificationDetailLoading, NotificationDetailScreen } from '@/widgets/notifications';
import { notificationDetailQueryOptions } from '@/features/notifications';
import { ServerPrefetchBoundary } from '@/shared/api/server-prefetch';
import { serverApiClient } from '@/shared/api/server-client';

/** Страница уведомления (карта #734, тикет #745): живые действия читателя,
 * прочтение — POST read при открытии, удаление — корзиной в баре. */
export const metadata: Metadata = {
  title: 'Уведомление — Рентли',
  description: 'Страница уведомления',
};

export default async function NotificationRoutePage({ params }: PageProps<'/notifications/[notificationId]'>) {
  const { notificationId } = await params;

  return (
    <Suspense fallback={<NotificationDetailLoading />}>
      <ServerPrefetchBoundary prefetch={(queryClient) => {
        void queryClient.prefetchQuery(notificationDetailQueryOptions({ id: notificationId, transport: serverApiClient }));
      }}>
        <NotificationDetailScreen notificationId={notificationId} />
      </ServerPrefetchBoundary>
    </Suspense>
  );
}
