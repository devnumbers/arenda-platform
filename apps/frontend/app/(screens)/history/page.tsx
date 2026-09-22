import type { Metadata } from 'next';
import { HistoryFeedScreen } from '@/widgets/history';

/** Лента «История действий» (карта #704, тикет #709): общая лента по всем
 * доступным объектам на едином хроме группы (screens) — оболочка
 * ScreenLayout, вход — строка на хабе «Совместный доступ» (решение
 * владельца 22.09). Новые снизу (мессенджер), догрузка старых — скроллом
 * вверх; поиск (#710), фильтры (#711), действия участника (#712) и
 * переходы строк (#713) — свои тикеты карты. */
export const metadata: Metadata = {
  title: 'История действий — Рентли',
  description: 'Лента действий по объектам',
};

export default function HistoryRoutePage() {
  return <HistoryFeedScreen />;
}
