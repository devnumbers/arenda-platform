import { Suspense } from 'react';
import type { JSX } from 'react';
import type { Metadata } from 'next';
import { HistoryFeedScreen, HistoryPropertyFeedSkeleton } from '@/widgets/history';

/** «Действия участника в объекте» (карта #838, тикет #841): та же лента
 * «Истории действий», прибитая к паре человек+объект — actor_ids и
 * property_ids по одному uuid (ADR 0061 §7; сервер AND'ит обе группы,
 * бэк #708 без изменений), группы фильтров адреса действуют поверх.
 * Вход — строка «Действия участника в объекте» на «Правах участника»
 * (макет 2177-59620). Идентификаторы кодируются в путь (Next декодирует
 * параметры обратно, канон ROUTES.historyMemberProperty); мусорный id
 * (не uuid) бэк отвечает 400, чужой/несуществующий uuid — privacy-404
 * всего запроса (канон ленты задач #547) — оба состояния экран рисует
 * канонным ErrorCard с «Повторить». Фильтры живут в query-параметрах,
 * поэтому клиентский экран со useSearchParams стоит за Suspense-
 * границей — требование App Router (прецедент /history). */
export const metadata: Metadata = {
  title: 'Действия участника в объекте — Рентли',
  description: 'Лента действий одного участника на одном объекте',
};

type HistoryMemberPropertyRoutePageProps = {
  params: Promise<{ participantId: string; propertyId: string }>;
};

export default async function HistoryMemberPropertyRoutePage({
  params,
}: HistoryMemberPropertyRoutePageProps): Promise<JSX.Element> {
  const { participantId, propertyId } = await params;

  return (
    <Suspense fallback={<HistoryPropertyFeedSkeleton />}>
      <HistoryFeedScreen participantId={participantId} propertyId={propertyId} />
    </Suspense>
  );
}
