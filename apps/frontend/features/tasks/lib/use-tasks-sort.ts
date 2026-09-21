'use client';

import { useState } from 'react';
import { DEFAULT_TASKS_SORT, serializeTasksSortToParams, type TasksSort } from './tasks-list';
import { useUrlParams } from '@/shared/lib/hooks/use-url-params';

/** Собственные параметры сортировки задач в адресе — знание этого модуля. */
const SORT_PARAMS = ['sort', 'order'] as const;

/**
 * Сортировка экранов задач в адресе (#785): ?sort=&order=, дефолт
 * («Дата, asc») не пишется — конвенция страницы «Объектов». Начальное
 * значение парсится на сервере (parseTasksSortParams). write снимает
 * sort/order перед записью: чужие параметры (фильтр ленты #524) не
 * затираются, дефолт снимается — в том числе протухшее значение из чужой
 * ссылки. Сам список хук не строит — группировка (groupTasks) остаётся
 * concern экрана.
 */
export function useTasksSort(initialSort?: TasksSort): {
  readonly sort: TasksSort;
  readonly changeSort: (next: TasksSort) => void;
} {
  const [sort, setSort] = useState<TasksSort>(initialSort ?? DEFAULT_TASKS_SORT);
  const { write } = useUrlParams();

  const changeSort = (next: TasksSort): void => {
    setSort(next);
    write(serializeTasksSortToParams(next), { own: SORT_PARAMS });
  };

  return { sort, changeSort };
}
