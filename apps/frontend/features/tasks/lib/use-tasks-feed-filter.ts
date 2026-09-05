'use client';

import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import {
  readTasksFeedFilter,
  tasksFeedFilterParams,
  tasksFeedPropertyParam,
  type TasksFeedFilter,
} from './tasks-feed-filter';

/**
 * Состояние фильтра по объекту в адресе ленты «Задачи» (#524): чтение —
 * readTasksFeedFilter (битые значения отбрасываются), запись — router.push,
 * поэтому «назад» по истории возвращает к ленте без фильтра. Открытие
 * страницы выбора объекта историю не трогает — черновик выбора живёт в
 * её состоянии (как у шитов операций, #477). `replace` — для авто-сброса
 * фильтра при 404 объекта (решение 8 #522): мёртвой ссылки в истории
 * оставаться не должно.
 */
export function useTasksFeedFilter(): {
  readonly filter: TasksFeedFilter;
  readonly applyPropertyFilter: (
    propertyId: string | null,
    options?: { readonly replace?: boolean },
  ) => void;
} {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const filter = readTasksFeedFilter(searchParams);

  const applyPropertyFilter = (
    propertyId: string | null,
    options?: { readonly replace?: boolean },
  ): void => {
    const params = new URLSearchParams(searchParams);
    params.delete(tasksFeedPropertyParam);
    for (const [name, value] of Object.entries(tasksFeedFilterParams({ propertyId }))) {
      params.set(name, value);
    }
    const queryString = params.toString();
    const url = queryString.length > 0 ? `${pathname}?${queryString}` : pathname;
    if (options?.replace === true) {
      router.replace(url, { scroll: false });
    } else {
      router.push(url, { scroll: false });
    }
  };

  return { filter, applyPropertyFilter };
}
