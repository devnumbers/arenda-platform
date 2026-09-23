import { Suspense } from 'react';
import type { JSX } from 'react';
import type { Metadata } from 'next';
import { HistoryFeedScreen, HistoryFeedSkeleton } from '@/widgets/history';

/** «Действия участника» (карта #704, тикет #712): та же лента «Истории
 * действий», прибитая к одному человеку — actor_ids = один uuid юзера
 * (ADR 0061 §7), группы фильтров адреса действуют поверх. Вход — тап по
 * актёру в общей ленте и кебаб страницы участника (#698). Идентификатор
 * кодируется в путь (Next декодирует параметр обратно, канон
 * ROUTES.participant); мусорный id (не uuid) бэк отвечает 400 — экран
 * рисует канонный ErrorCard с «Повторить». Фильтры живут в
 * query-параметрах, поэтому клиентский экран со useSearchParams стоит за
 * Suspense-границей — требование App Router (прецедент /history). */
export const metadata: Metadata = {
  title: 'Действия участника — Рентли',
  description: 'Лента действий одного участника',
};

type HistoryParticipantRoutePageProps = {
  params: Promise<{ participantId: string }>;
};

export default async function HistoryParticipantRoutePage({
  params,
}: HistoryParticipantRoutePageProps): Promise<JSX.Element> {
  const { participantId } = await params;

  return (
    <Suspense fallback={<HistoryFeedSkeleton />}>
      <HistoryFeedScreen participantId={participantId} />
    </Suspense>
  );
}
