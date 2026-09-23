/**
 * Отображение вида действия (CONTEXT: «Вид действия»): каноническая
 * подпись каждой из семи групп словаря журнала (ADR 0061 §4) — она же
 * группа чекбоксов фильтра «Виды действий» (тикет #711). Тон — слой
 * отображения, как у base-action-view.
 */

import type { HistoryKind } from './types';

/** Все виды словаря в каноническом порядке (макет 2050-158280). */
export const HISTORY_KINDS: ReadonlyArray<HistoryKind> = [
  'property',
  'rental',
  'payment',
  'operation',
  'contact',
  'task',
  'member',
];

export function kindLabel(kind: HistoryKind): string {
  switch (kind) {
    case 'property':
      return 'Объект';
    case 'rental':
      return 'Аренда';
    case 'payment':
      return 'Платежи';
    case 'operation':
      return 'Операции';
    case 'contact':
      return 'Контакты';
    case 'task':
      return 'Задачи';
    case 'member':
      return 'Участники';
  }
}
