/**
 * Отображение основного действия (CONTEXT: «Основное действие»): каждая
 * из четырёх групп — свой цвет иконки строки и каноническая подпись
 * (она же группа чекбоксов фильтра, тикет #711). Тон — слой отображения:
 * строка мапит его на иконку канона и текстовый класс.
 */

import type { HistoryBaseAction } from './types';

export type HistoryBaseActionTone = 'success' | 'warning' | 'primary' | 'danger';

export function baseActionTone(baseAction: HistoryBaseAction): HistoryBaseActionTone {
  switch (baseAction) {
    case 'added':
      return 'success';
    case 'changed':
      return 'warning';
    case 'completed':
      return 'primary';
    case 'deleted':
      return 'danger';
  }
}

export function baseActionLabel(baseAction: HistoryBaseAction): string {
  switch (baseAction) {
    case 'added':
      return 'Добавление';
    case 'changed':
      return 'Изменение';
    case 'completed':
      return 'Выполнение';
    case 'deleted':
      return 'Удаление';
  }
}
