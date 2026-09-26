import type { Metadata } from 'next';
import { NotificationDetailScreen } from '@/widgets/notifications';

/** Страница уведомления (карта #734, тикет #745): живые действия читателя,
 * прочтение — POST read при открытии, удаление — корзиной в баре. */
export const metadata: Metadata = {
  title: 'Уведомление — Рентли',
  description: 'Страница уведомления',
};

export default async function NotificationRoutePage({ params }: PageProps<'/notifications/[notificationId]'>) {
  const { notificationId } = await params;

  return <NotificationDetailScreen notificationId={notificationId} />;
}
