import { isActiveTasksFeedFilter, type TasksFeedFilter } from '@/features/tasks';

/**
 * Гейт пустых состояний глобальной ленты «Задачи» (#523): одно решение —
 * что показывает экран, когда обе выборки (активные и выполненные) пусты.
 * Полная пустота без фильтра — «Задач нет» с CTA, чипы и «+» спрятаны
 * (анатомия 3235-74057); пустота под фильтром «Объект» — минимальное
 * «Задач не найдено» (канон OperationsEmptyPeriod #474): чипы и «+»
 * остаются, фильтр меняют тем же чипом. Не пусто или выборки ещё не
 * сошлись — 'none': лента, скелетон и ошибки решает экран.
 */
export type TasksFeedEmptyState = 'none' | 'feed' | 'filtered';

export function tasksFeedEmptyState({
  filter,
  loaded,
  hasTasks,
}: {
  readonly filter: TasksFeedFilter;
  readonly loaded: boolean;
  readonly hasTasks: boolean;
}): TasksFeedEmptyState {
  if (!loaded || hasTasks) {
    return 'none';
  }
  return isActiveTasksFeedFilter(filter) ? 'filtered' : 'feed';
}
