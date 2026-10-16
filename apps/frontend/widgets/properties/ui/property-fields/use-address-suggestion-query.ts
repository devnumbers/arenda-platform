'use client';

import { useAddressSuggestions } from '@/features/properties';
import { useDebounce } from '@/shared/lib/hooks/useDebounce';

/**
 * Общий запрос подсказок адреса DaData для шага «Адрес» визарда и поиска
 * адреса формы правки (#590): дебаунс 300мс, подсказки живут с 3
 * символов (константа экспортируется для гейта списка). Ошибка сети
 * тихая — списка просто нет, ручной ввод продолжает работать.
 */

export const SUGGEST_QUERY_MIN_LENGTH = 3;

export function useAddressSuggestionQuery(rawValue: string): {
  /** Запрос после дебаунса — условие «поиска» у вызывающего. */
  readonly debouncedQuery: string;
  /** Строки подсказок; undefined, пока ответа нет. */
  readonly suggestions: ReadonlyArray<{ readonly value: string; readonly city?: string }> | undefined;
} {
  const debouncedQuery = useDebounce(rawValue, 300);
  const { data: suggestions } = useAddressSuggestions(debouncedQuery);
  return { debouncedQuery, suggestions };
}
