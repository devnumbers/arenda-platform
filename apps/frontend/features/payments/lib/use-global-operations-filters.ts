'use client';

import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { clientTodayIso } from '@/entities/payment';
import {
  globalOperationsFiltersParams,
  readGlobalOperationsFilters,
  type GlobalOperationsFilters,
} from './operations-global-filters';
import type { OperationsPeriod } from './operations-filters';

/**
 * Состояние фильтров глобальной ленты «Операции» в адресе (#541): те же
 * правила, что на объектном экране (#477) — чтение отбрасывает битые
 * значения, запись через router.push, «назад» возвращает к прежнему
 * списку; плюс мультивыбор объектов (#539, ?property=csv). Пикер периода
 * пишет с заменой записи истории; дефолт — текущий месяц, все объекты,
 * «Все категории».
 */
export function useGlobalOperationsFilters(): {
  readonly filters: GlobalOperationsFilters;
  readonly applyPeriod: (
    period: OperationsPeriod,
    options?: { readonly replace?: boolean },
  ) => void;
  readonly applyCategories: (categories: ReadonlyArray<string>) => void;
  readonly applyPropertyIds: (propertyIds: ReadonlyArray<string>) => void;
} {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const filters = readGlobalOperationsFilters(searchParams, clientTodayIso());

  const apply = (next: GlobalOperationsFilters, replace: boolean): void => {
    const params = new URLSearchParams(searchParams);
    params.delete('from');
    params.delete('to');
    params.delete('category');
    params.delete('property');
    for (const [name, value] of Object.entries(globalOperationsFiltersParams(next))) {
      params.set(name, value);
    }
    const queryString = params.toString();
    const url = queryString.length > 0 ? `${pathname}?${queryString}` : pathname;
    if (replace) {
      router.replace(url, { scroll: false });
    } else {
      router.push(url, { scroll: false });
    }
  };

  return {
    filters,
    applyPeriod: (period, options) =>
      apply(
        { period, categories: filters.categories, propertyIds: filters.propertyIds },
        options?.replace === true,
      ),
    applyCategories: (categories) =>
      apply({ period: filters.period, categories, propertyIds: filters.propertyIds }, false),
    applyPropertyIds: (propertyIds) =>
      apply({ period: filters.period, categories: filters.categories, propertyIds }, false),
  };
}
