/**
 * Отображение вида действия (CONTEXT: «Вид действия»): каноническая
 * подпись каждой из семи групп словаря журнала (ADR 0061 §4) — она же
 * группа чекбоксов фильтра «Виды действий» (тикет #711). Тон — слой
 * отображения, как у base-action-view.
 */

import type { HistoryBaseAction, HistoryKind } from './types';

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

/** Все основные действия в каноническом порядке (ADR 0061 §4): каталог
 * группы чекбоксов фильтра «Основные действия» (#711) и семантики
 * «все, кроме переключённой» (null → список). */
export const HISTORY_BASE_ACTIONS: ReadonlyArray<HistoryBaseAction> = [
  'added',
  'changed',
  'completed',
  'deleted',
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
