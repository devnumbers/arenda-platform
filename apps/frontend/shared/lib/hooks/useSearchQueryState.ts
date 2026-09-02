'use client';

import { useCallback, useState } from 'react';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { useDebounce } from './useDebounce';

/**
 * Состояние поисковой строки новых экранов (переиспользуемая часть поиска
 * операций, #476): ввод живёт в состоянии и сразу отражается в адресе —
 * query-параметр `q` обновляется через replace без скролла и без записей в
 * истории, поэтому перезагрузка и «поделиться» сохраняют запрос. Серверный
 * запрос догоняет ввод с задержкой `debounceMs` (хук useDebounce): значение
 * `debounced` — ключ запроса, `value` — что видно в поле. Сам серверный
 * запрос хук не делает — это concern фичи (react-query хуки); экран
 * compose'ит этот хук со своими запросами.
 */
export function useSearchQueryState(
  queryParam = 'q',
  debounceMs = 300,
): {
  readonly value: string;
  readonly debounced: string;
  readonly setValue: (value: string) => void;
  readonly clear: () => void;
} {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const [value, setValueState] = useState(() => searchParams.get(queryParam) ?? '');

  const setValue = useCallback(
    (next: string) => {
      setValueState(next);
      const params = new URLSearchParams(searchParams);
      if (next.length > 0) {
        params.set(queryParam, next);
      } else {
        params.delete(queryParam);
      }
      const queryString = params.toString();
      router.replace(queryString.length > 0 ? `${pathname}?${queryString}` : pathname, {
        scroll: false,
      });
    },
    [pathname, queryParam, router, searchParams],
  );

  const clear = useCallback(() => setValue(''), [setValue]);
  const debounced = useDebounce(value, debounceMs);

  return { value, debounced, setValue, clear };
}
