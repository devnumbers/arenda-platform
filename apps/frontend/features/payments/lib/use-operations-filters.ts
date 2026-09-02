'use client';

import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { clientTodayIso } from '@/entities/payment';
import {
  operationsFiltersParams,
  readOperationsFilters,
  type OperationsFilters,
  type OperationsPeriod,
} from './operations-filters';

/**
 * Состояние фильтров период/категории в адресе страницы (#477): чтение —
 * readOperationsFilters (битые значения отбрасываются), запись — router.push,
 * поэтому «назад» по истории возвращает к списку без фильтра, а ссылка с
 * ?from=&to=&category= восстанавливает выбор. Открытие шитов историю не
 * трогает — черновик выбора живёт внутри шита. Дефолт (без параметров) —
 * текущий месяц, «Все категории».
 */
export function useOperationsFilters(): {
  readonly filters: OperationsFilters;
  readonly applyPeriod: (period: OperationsPeriod) => void;
  readonly applyCategories: (categories: ReadonlyArray<string>) => void;
} {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const filters = readOperationsFilters(searchParams, clientTodayIso());

  const apply = (next: OperationsFilters): void => {
    const params = new URLSearchParams(searchParams);
    params.delete('from');
    params.delete('to');
    params.delete('category');
    for (const [name, value] of Object.entries(operationsFiltersParams(next))) {
      params.set(name, value);
    }
    const queryString = params.toString();
    router.push(queryString.length > 0 ? `${pathname}?${queryString}` : pathname, {
      scroll: false,
    });
  };

  return {
    filters,
    applyPeriod: (period) => apply({ period, categories: filters.categories }),
    applyCategories: (categories) => apply({ period: filters.period, categories }),
  };
}
