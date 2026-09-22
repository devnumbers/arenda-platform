import type { Metadata } from 'next';
import { parseStringParam } from '@/shared/lib/parse-string-param';
import { NotificationsFeedScreen } from '@/widgets/notifications';

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

  return <NotificationsFeedScreen initialUnreadOnly={initialUnreadOnly} />;
}
