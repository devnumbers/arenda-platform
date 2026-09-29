'use client';

import { dateToIsoLocal } from '@/shared/lib/calendar';
import { useUrlParams } from '@/shared/lib/hooks/use-url-params';
import {
  globalOperationsFiltersParams,
  readGlobalOperationsFilters,
  type GlobalOperationsFilters,
} from './operations-global-filters';
import type { OperationsPeriod } from './operations-filters';

/**
 * Состояние фильтров глобальной ленты «Операции» в адресе (#541): те же
 * правила, что на объектном экране (#477) — чтение отбрасывает битые
 * значения, запись через router.push (useUrlParams, #786), «назад»
 * возвращает к прежнему списку; плюс мультивыбор объектов (#539,
 * ?property=csv). Пикер периода пишет с заменой записи истории; null
 * (#670) — сброс периода: from/to уходят из URL, лента открывается весь
 * период. Дефолт — весь период, все объекты, «Все категории».
 */

/** Собственные параметры фильтров глобальной ленты — знание этого модуля. */
const FILTER_PARAMS = ['from', 'to', 'category', 'property', 'archived'] as const;

export function useGlobalOperationsFilters(): {
  readonly filters: GlobalOperationsFilters;
  readonly applyPeriod: (
    period: OperationsPeriod | null,
    options?: { readonly replace?: boolean },
  ) => void;
  readonly applyCategories: (categories: ReadonlyArray<string>) => void;
  readonly applyPropertyIds: (propertyIds: ReadonlyArray<string>) => void;
} {
  const { params, write } = useUrlParams();

  const filters = readGlobalOperationsFilters(params, dateToIsoLocal(new Date()));

  const apply = (next: GlobalOperationsFilters, replace: boolean): void => {
    write(globalOperationsFiltersParams(next), {
      own: FILTER_PARAMS,
      mode: replace ? 'replace' : 'push',
    });
  };

  return {
    filters,
    applyPeriod: (period, options) =>
      apply(
        {
          period,
          categories: filters.categories,
          propertyIds: filters.propertyIds,
          archived: filters.archived,
        },
        options?.replace === true,
      ),
    applyCategories: (categories) =>
      apply(
        {
          period: filters.period,
          categories,
          propertyIds: filters.propertyIds,
          archived: filters.archived,
        },
        false,
      ),
    applyPropertyIds: (propertyIds) =>
      apply(
        {
          period: filters.period,
          categories: filters.categories,
          propertyIds,
          archived: filters.archived,
        },
        false,
      ),
  };
}
