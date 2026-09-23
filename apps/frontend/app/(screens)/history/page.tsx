import { Suspense } from 'react';
import type { Metadata } from 'next';
import { HistoryFeedScreen, HistoryFeedSkeleton } from '@/widgets/history';

/** Лента «История действий» (карта #704, тикеты #709–#711): общая лента
 * по всем доступным объектам на едином хроме группы (screens) — оболочка
 * ScreenLayout, вход — строка на хабе «Совместный доступ» (решение
 * владельца 22.09). Новые снизу (мессенджер), догрузка старых — скроллом
 * вверх; поиск (#710), фильтры (#711), действия участника (#712) и
 * переходы строк (#713) — свои тикеты карты. Фильтры живут в
 * query-параметрах, поэтому клиентский экран со useSearchParams стоит за
 * Suspense-границей — требование App Router (прецедент /operations). */
export const metadata: Metadata = {
  title: 'История действий — Рентли',
  description: 'Лента действий по объектам',
};

export default function HistoryRoutePage() {
  return (
    <Suspense fallback={<HistoryFeedSkeleton />}>
      <HistoryFeedScreen />
    </Suspense>
  );
}
