'use client';

import { dateToIsoLocal } from '@/shared/lib/calendar';
import { useUrlParams } from '@/shared/lib/hooks/use-url-params';
import {
  historyFiltersParams,
  readHistoryFilters,
  type HistoryFilters,
} from './history-filters';

/**
 * Состояние фильтров ленты «Истории» в адресе (#711): чтение отбрасывает
 * битые значения (канон #477), «Применить фильтры» пишет patch одним
 * push'ом (useUrlParams #786) — «назад» возвращает ленту без фильтров
 * (канон «Состояние страницы в адресе», DESIGN.md §3). Дефолт — весь
 * период, все группы «все» (аннотация макета 2177-60527). Фильтры
 * перечитываются из адреса на каждый рендер без мемоизации — чтение
 * чистое и дешёвое, как у useOperationsFilters.
 */

/** Собственные параметры фильтров истории — знание этого модуля. */
const FILTER_PARAMS = ['from', 'to', 'actions', 'kinds', 'actors', 'objects'] as const;

export function useHistoryFiltersState(): {
  readonly filters: HistoryFilters;
  readonly applyFilters: (next: HistoryFilters) => void;
} {
  const { params, write } = useUrlParams();

  const filters = readHistoryFilters(params, dateToIsoLocal(new Date()));

  const applyFilters = (next: HistoryFilters): void => {
    write(historyFiltersParams(next), { own: FILTER_PARAMS, mode: 'push' });
  };

  return { filters, applyFilters };
}
