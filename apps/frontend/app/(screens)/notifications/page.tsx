import type { Metadata } from 'next';
import { NotificationsFeedScreen } from '@/widgets/notifications';

/** Центр уведомлений — лента (карта #734, тикет #744). На едином хроме:
 * оболочка — ScreenLayout группы (screens), вход — средний таб TabBar
 * мобайла и пилюля «Уведомления» ПК. Фильтр «Непрочитанные» живёт в query
 * строки (?unread=1) — переживает перезагрузку. */
export const metadata: Metadata = {
  title: 'Уведомления — Рентли',
  description: 'Центр уведомлений',
};

type NotificationsRoutePageProps = {
  searchParams?: Promise<Record<string, string | string[] | undefined>>;
};

export default async function NotificationsRoutePage({
  searchParams,
}: NotificationsRoutePageProps) {
  const resolved = searchParams ? await searchParams : {};
  const unreadParam = resolved.unread;
  const unreadValue = Array.isArray(unreadParam) ? unreadParam[0] : unreadParam;
  const initialUnreadOnly = unreadValue === '1';

  return <NotificationsFeedScreen initialUnreadOnly={initialUnreadOnly} />;
}
