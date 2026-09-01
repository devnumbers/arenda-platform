'use client';

import { useEffect, useRef, useState } from 'react';
import type { ChangeEvent, JSX, KeyboardEvent } from 'react';
import {
  addressSuggestionRow,
  useAddressSuggestions,
} from '@/features/properties';
import { useDebounce } from '@/shared/lib/hooks/useDebounce';
import { ListRow, TextField } from '@/shared/ui/design';

/**
 * Шаг 2 «Адрес» (Figma 1213-52017/52391, 1519-94336): поле с программным
 * фокусом на входе в шаг (autoFocus запрещён — паттерн CodeStep) и
 * подсказками DaData строками под полем. Подсказки живут с 3 символов
 * (дебаунс 300мс, как в легаси-AddressField); ошибка сети тихая — списка
 * просто нет, ручной ввод продолжает работать. Выбор подсказки
 * фиксирует полный адрес в черновике и прячет список до следующей правки.
 */

const SUGGEST_QUERY_MIN_LENGTH = 3;

export type AddressStepProps = {
  readonly value: string;
  readonly onChange: (address: string) => void;
};

export function AddressStep({ value, onChange }: AddressStepProps): JSX.Element {
  const inputRef = useRef<HTMLInputElement | null>(null);
  const listRef = useRef<HTMLDivElement | null>(null);
  // Восстановленный адрес (возврат «назад») подсказки не вызывает —
  // список появится после следующей правки значения.
  const [dismissed, setDismissed] = useState(value.trim().length > 0);
  const [inputValue, setInputValue] = useState(value);
  const debouncedQuery = useDebounce(inputValue, 300);
  const { data: suggestions } = useAddressSuggestions(debouncedQuery);

  // Программный фокус при входе в шаг.
  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  const rows = suggestions ?? [];
  const query = debouncedQuery.trim();
  const showList =
    !dismissed && query.length >= SUGGEST_QUERY_MIN_LENGTH && rows.length > 0;

  return (
    <>
      <div className="px-6 pt-6">
        <TextField
          ref={inputRef}
          title="Введите адрес"
          value={inputValue}
          onChange={handleChange}
          onKeyDown={handleKeyDown}
          onClear={clear}
        />
      </div>
      {showList && (
        <div ref={listRef} className="mt-2">
          {rows.map((suggestion) => {
            const row = addressSuggestionRow(suggestion.value, suggestion.city);
            return (
              <ListRow
                key={suggestion.value}
                className="px-3 py-3"
                subtitleClassName="text-content-secondary"
                title={row.title}
                subtitle={row.subtitle}
                onSelect={() => select(suggestion.value)}
              />
            );
          })}
        </div>
      )}
    </>
  );

  function handleChange(event: ChangeEvent<HTMLInputElement>): void {
    setInputValue(event.currentTarget.value);
    setDismissed(false);
    onChange(event.currentTarget.value);
  }

  function handleKeyDown(event: KeyboardEvent<HTMLInputElement>): void {
    if (event.key === 'Escape') {
      setDismissed(true);
      return;
    }
    if (event.key === 'ArrowDown' && showList) {
      event.preventDefault();
      listRef.current?.querySelector<HTMLElement>('[role="button"]')?.focus();
    }
  }

  function select(address: string): void {
    setInputValue(address);
    setDismissed(true);
    onChange(address);
    inputRef.current?.focus();
  }

  function clear(): void {
    setInputValue('');
    setDismissed(false);
    onChange('');
    inputRef.current?.focus();
  }
}
