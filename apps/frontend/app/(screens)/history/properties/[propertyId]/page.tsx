import { Suspense } from 'react';
import type { JSX } from 'react';
import type { Metadata } from 'next';
import { HistoryFeedLoading, HistoryFeedScreen, HistoryObjectGate } from '@/widgets/history';

/** «История объекта» (карта #838, тикет #840): та же лента «Истории
 * действий», прибитая к одному объекту — property_ids = один uuid
 * (ADR 0061 §7), группы фильтров адреса действуют поверх. Вход — кебаб
 * «Участников объекта» (макет 1980-139712). Идентификатор кодируется в
 * путь (Next декодирует параметр обратно, канон ROUTES.historyProperty);
 * мусорный id (не uuid) бэк отвечает 400, чужой/несуществующий uuid —
 * privacy-404 всего запроса (канон ленты задач #547) — оба состояния
 * экран рисует канонным ErrorCard с «Повторить». Фильтры живут в
 * query-параметрах, поэтому клиентский экран со useSearchParams стоит за
 * Suspense-границей — требование App Router (прецедент /history). Фолбэк
 * границы — с хромом экрана (#1281, карта #1324): скелетон без шапки
 * «прыгал» в хром при приходе контента. Страница под тарифной разводкой
 * #1466: proxy-паттерн объектных поверхностей снят — участник объекта
 * читает на любом тарифе, владельца на basic уводит HistoryObjectGate. */
export const metadata: Metadata = {
  title: 'История объекта — Рентли',
  description: 'Лента действий одного объекта',
};

export default async function HistoryPropertyRoutePage({
  params,
}: PageProps<'/history/properties/[propertyId]'>): Promise<JSX.Element> {
  const { propertyId } = await params;

  return (
    <HistoryObjectGate propertyId={propertyId}>
      <Suspense fallback={<HistoryFeedLoading propertyId={propertyId} />}>
        <HistoryFeedScreen propertyId={propertyId} />
      </Suspense>
    </HistoryObjectGate>
  );
}
