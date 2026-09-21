import type { Metadata } from 'next';
import { NotificationDetailScreen } from '@/widgets/notifications';

/** Страница уведомления (карта #734, тикет #745): живые действия читателя,
 * прочтение — POST read при открытии, удаление — корзиной в баре. */
export const metadata: Metadata = {
  title: 'Уведомление — Рентли',
  description: 'Страница уведомления',
};

type NotificationRoutePageProps = {
  params: Promise<{ notificationId: string }>;
};

export default async function NotificationRoutePage({ params }: NotificationRoutePageProps) {
  const { notificationId } = await params;

  return <NotificationDetailScreen notificationId={notificationId} />;
}
