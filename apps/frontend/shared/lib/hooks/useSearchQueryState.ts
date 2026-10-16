'use client';

import { useState } from 'react';
import { useDebounce } from './useDebounce';
import { useUrlParams } from './use-url-params';

/**
 * Состояние поисковой строки новых экранов (переиспользуемая часть поиска
 * операций, #476): ввод живёт в состоянии и сразу отражается в адресе —
 * query-параметр `q` обновляется через replace без скролла и без записей в
 * истории, поэтому перезагрузка и «поделиться» сохраняют запрос. Запись —
 * канон useUrlParams (#786: ручная сборка URLSearchParams снята, ядро
 * хука дословно совпадало с каноном); `own` — только свой параметр, чужие
 * параметры адреса переживают ввод. Серверный запрос догоняет ввод с
 * задержкой `debounceMs` (хук useDebounce): значение `debounced` — ключ
 * запроса, `value` — что видно в поле. Сам серверный запрос хук не
 * делает — это concern фичи (react-query хуки); экран compose'ит этот хук
 * со своими запросами.
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
  const { params, write } = useUrlParams();

  const [value, setValueState] = useState(() => params.get(queryParam) ?? '');

  const setValue = (next: string): void => {
    setValueState(next);
    write(next.length > 0 ? { [queryParam]: next } : {}, { own: [queryParam] });
  };

  const clear = (): void => setValue('');
  const debounced = useDebounce(value, debounceMs);

  return { value, debounced, setValue, clear };
}
