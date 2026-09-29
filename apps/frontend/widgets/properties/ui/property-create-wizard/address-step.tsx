'use client';

import { useEffect, useRef, useState } from 'react';
import type { ChangeEvent, JSX, KeyboardEvent } from 'react';
import { focusListboxEdge } from '@/shared/ui/design/listbox-keyboard';
import { TextField } from '@/shared/ui/design';
import { AddressSuggestionList } from '../property-fields/address-suggestion-list';
import {
  SUGGEST_QUERY_MIN_LENGTH,
  useAddressSuggestionQuery,
} from '../property-fields/use-address-suggestion-query';

/**
 * Шаг 2 «Адрес» (Figma 1213-52017/52391, 1519-94336): поле с программным
 * фокусом на входе в шаг (autoFocus запрещён — паттерн CodeStep) и
 * подсказками DaData строками под полем (общий список с поиском адреса
 * формы правки, property-fields). Подсказки живут с 3 символов (дебаунс
 * 300мс, как в легаси-AddressField); ошибка сети тихая — списка просто
 * нет, ручной ввод продолжает работать. Выбор подсказки фиксирует полный
 * адрес в черновике и прячет список до следующей правки.
 */

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
  const { debouncedQuery, suggestions } = useAddressSuggestionQuery(inputValue);

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
        <AddressSuggestionList
          containerRef={listRef}
          containerClassName="mt-2"
          rowClassName="py-3"
          suggestions={rows}
          onSelect={select}
          onEscape={hideList}
        />
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
    if (showList && event.key === 'ArrowDown') {
      event.preventDefault();
      focusListboxEdge(listRef.current, 'first');
    }
    if (showList && event.key === 'ArrowUp') {
      event.preventDefault();
      focusListboxEdge(listRef.current, 'last');
    }
  }

  // Escape на строке всплывает с опции (runListboxAction его не гасит):
  // список прячется, фокус возвращается в поле.
  function hideList(): void {
    setDismissed(true);
    inputRef.current?.focus();
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
