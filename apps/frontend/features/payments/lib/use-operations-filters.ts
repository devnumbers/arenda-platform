'use client';

import { dateToIsoLocal } from '@/shared/lib/calendar';
import { useUrlParams } from '@/shared/lib/hooks/use-url-params';
import {
  operationsFiltersParams,
  readOperationsFilters,
  type OperationsFilters,
  type OperationsPeriod,
} from './operations-filters';

/**
 * Состояние фильтров период/категории в адресе страницы (#477): чтение —
 * readOperationsFilters (битые значения отбрасываются), запись — router.push
 * через useUrlParams (#786), поэтому «назад» по истории возвращает к списку
 * без фильтра, а ссылка с ?from=&to=&category= восстанавливает выбор.
 * Открытие шитов историю не трогает — черновик выбора живёт внутри шита.
 * null (#674) — сброс периода: from/to уходят из URL; дефолт (без
 * параметров) на всех экранах операций объекта — весь период (карта #669).
 */

/** Собственные параметры фильтров операций в адресе — знание этого модуля. */
const FILTER_PARAMS = ['from', 'to', 'category'] as const;

export function useOperationsFilters(): {
  readonly filters: OperationsFilters;
  /**
   * Запись периода в адрес; `replace` — заменить запись истории вместо
   * новой (пикер периода: применений в истории не остаётся, как на
   * прежней странице периода). null — сброс периода (#674), как в
   * глобальном хуке (#670).
   */
  readonly applyPeriod: (
    period: OperationsPeriod | null,
    options?: { readonly replace?: boolean },
  ) => void;
  readonly applyCategories: (categories: ReadonlyArray<string>) => void;
} {
  const { params, write } = useUrlParams();

  const filters = readOperationsFilters(params, dateToIsoLocal(new Date()));

  const apply = (next: OperationsFilters, replace: boolean): void => {
    write(operationsFiltersParams(next), {
      own: FILTER_PARAMS,
      mode: replace ? 'replace' : 'push',
    });
  };

  return {
    filters,
    applyPeriod: (period, options) =>
      apply({ period, categories: filters.categories }, options?.replace === true),
    applyCategories: (categories) => apply({ period: filters.period, categories }, false),
  };
}
