'use client';

import { useState } from 'react';
import { DEFAULT_TASKS_SORT, serializeTasksSortToParams, type TasksSort } from './tasks-list';
import { useUrlParams } from '@/shared/lib/hooks/use-url-params';

/** Собственные параметры сортировки задач в адресе — знание этого модуля. */
const SORT_PARAMS = ['sort', 'order'] as const;

/**
 * Сортировка экранов задач в адресе (#785): ?sort=&order=, дефолт
 * («Дата, asc») не пишется — конвенция состояния в адресе. Начальное
 * значение парсится на сервере (parseTasksSortParams). write снимает
 * sort/order перед записью: чужие параметры (фильтр ленты #524) не
 * затираются, дефолт снимается — в том числе протухшее значение из чужой
 * ссылки. Сам список хук не строит — группировка (groupTasks) остаётся
 * concern экрана.
 *
 * Известное принятое расхождение (#786, осознанное «оставляем»): фильтр
 * #524 пишет push-записи в ту же страницу, поэтому «назад» в её пределах
 * меняет searchParams БЕЗ ремоунта экрана, а зеркало useState новое
 * initialSort игнорирует — сортировка на экране может разойтись с адресом
 * до перезагрузки (унаследовано от канона PropertiesPage). Принято
 * намеренно: зеркало даёт мгновенную пересортировку без ожидания
 * RSC-раундтрипа, которым отвечает смена searchParams (производное от
 * адреса состояние отложило бы чип за этим запросом); ресинк-эффект через
 * setState в эффекте запрещён баром качества (react-hooks v7); расхождение
 * достижимо только при push-записи в ту же страницу (сегодня — лишь
 * фильтр /tasks), перезагрузка всегда восстанавливает сортировку из
 * адреса; replace-писатели (сортировки) записей same-path в свою страницу
 * не создают вовсе.
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
