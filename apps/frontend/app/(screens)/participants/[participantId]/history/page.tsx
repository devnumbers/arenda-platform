import { Suspense } from 'react';
import type { JSX } from 'react';
import type { Metadata } from 'next';
import {
  HistoryFeedLoading,
  HistoryFeedScreen,
  HistoryParticipantGate,
} from '@/widgets/history';

/** «Действия участника» (карта #1582, решение владельца 22.10.2026 в
 * переработке): лента одного человека по всем его текущим ногам доступа —
 * скоуп «человек × его ноги» выводит сервер (GET
 * /participants/{participantId}/history, ADR 0061 §7), чужих людей и
 * чужих объектов в ленте нет по построению. Идентификатор кодируется в
 * путь — uuid юзера либо pending-почта (канон ROUTES.participant; Next
 * декодирует параметр обратно, билдер запроса кодирует сам). Не в скоупе
 * читающего — privacy-404 всего запроса (канон ленты задач #547), экран
 * рисует канонный ErrorCard с «Повторить»; битый id — 400. Не путь
 * /history/participants/[id]: там перманентный 308 #1361. Страница под
 * тарифной разводкой (зеркало #1466): поверхность владельца, basic
 * уводит HistoryParticipantGate на интерстишал — роль-осознанный гейт
 * бекенда (ревизия #1465) отвечает 402 его же canonical-телом. Шит
 * «Настройки» — как на всех скоуп-страницах (аменда владельца 22.10.2026):
 * «Участник» прибит серым, «Объекты» предлагает ноги агрегата, выбор
 * сужает ленту пересечением на сервере; фильтры в query-параметрах
 * адреса (§3), пины в URL не живут. Клиентский экран стоит за
 * Suspense-границей — требование App Router для useSearchParams-экранов
 * (прецедент /history); фолбэк границы — с хромом экрана (#1281, карта
 * #1324): скелетон без шапки «прыгал» в хром при приходе контента. */
export const metadata: Metadata = {
  title: 'Действия участника — Рентли',
  description: 'Лента действий одного участника по всем его объектам',
};

export default async function ParticipantHistoryRoutePage({
  params,
}: PageProps<'/participants/[participantId]/history'>): Promise<JSX.Element> {
  const { participantId } = await params;

  return (
    <HistoryParticipantGate>
      <Suspense fallback={<HistoryFeedLoading participantId={participantId} />}>
        <HistoryFeedScreen participantId={participantId} />
      </Suspense>
    </HistoryParticipantGate>
  );
}
