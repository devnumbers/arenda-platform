'use client';

import {
  readTasksFeedFilter,
  tasksFeedFilterParams,
  tasksFeedPropertyParam,
  tasksFeedWithoutPropertyParam,
  type TasksFeedFilter,
} from './tasks-feed-filter';
import { useUrlParams } from '@/shared/lib/hooks/use-url-params';

/**
 * Состояние фильтра ленты «Задачи» в адресе (#524, «Общие задачи» —
 * решение владельца 2026-09-07): чтение — readTasksFeedFilter (битые
 * значения отбрасываются), запись — router.push через useUrlParams
 * (#786), поэтому «назад» по истории возвращает к ленте без фильтра.
 * Открытие страницы выбора объекта историю не трогает — черновик выбора
 * живёт в её состоянии (как у шитов операций, #477). `replace` — для
 * авто-сброса фильтра при 404 (решение 8 #522): мёртвой ссылки в истории
 * оставаться не должно.
 */

/** Собственные параметры фильтра в адресе — знание этого модуля. */
const FILTER_PARAMS = [tasksFeedPropertyParam, tasksFeedWithoutPropertyParam] as const;

export function useTasksFeedFilter(): {
  readonly filter: TasksFeedFilter;
  readonly applyFeedFilter: (
    filter: TasksFeedFilter,
    options?: { readonly replace?: boolean },
  ) => void;
} {
  const { params, write } = useUrlParams();

  const filter = readTasksFeedFilter(params);

  const applyFeedFilter = (
    next: TasksFeedFilter,
    options?: { readonly replace?: boolean },
  ): void => {
    write(tasksFeedFilterParams(next), {
      own: FILTER_PARAMS,
      mode: options?.replace === true ? 'replace' : 'push',
    });
  };

  return { filter, applyFeedFilter };
}
