'use client';

import { useEffect, useRef, useState } from 'react';
import type { ChangeEvent, JSX, KeyboardEvent } from 'react';
import Image from 'next/image';
import { focusListboxEdge } from '@/shared/ui/design/listbox-keyboard';
import { IconButton, PageContent, SearchField, TopNav } from '@/shared/ui/design';
import { Cancel } from '@/shared/assets/icons';
import { AddressSuggestionList } from '../property-fields/address-suggestion-list';
import {
  SUGGEST_QUERY_MIN_LENGTH,
  useAddressSuggestionQuery,
} from '../property-fields/use-address-suggestion-query';

/**
 * Полноэкранный поиск адреса формы правки (карта #583, тикет #590;
 * Figma 1518:93118 пустое, 1518:93341 подсказки). Поверхность —
 * полноэкранный оверлей (DESIGN.md §4, как CalendarDatePicker): состояние
 * формы не теряется. Шапка — поисковая (TopNav variant search): крестик
 * слева закрывает, поле с программным фокусом; значение обновляет адрес
 * формы с первого символа (ручной ввод работает и без подсказок, как на
 * шаге «Адрес» визарда), подсказки — с 3 символов (дебаунс 300мс),
 * выбор фиксирует полный адрес и закрывает. Пустые состояния —
 * иллюстрация 128: «Введите адрес объекта» до поиска, «Адрес не найден»
 * на пустой результат (канон состояний поиска, строка серым 16/18).
 */

export type PropertyAddressSearchProps = {
  /** Текущий адрес формы — стартовое значение поля. */
  readonly value: string;
  /** Живая правка адреса из поля (ручной ввод без выбора подсказки). */
  readonly onChange: (address: string) => void;
  /** Выбор подсказки: полный адрес + закрыть. */
  readonly onSelect: (address: string) => void;
  readonly onClose: () => void;
};

export function PropertyAddressSearch({
  value,
  onChange,
  onSelect,
  onClose,
}: PropertyAddressSearchProps): JSX.Element {
  const inputRef = useRef<HTMLInputElement | null>(null);
  const listRef = useRef<HTMLDivElement | null>(null);
  const [inputValue, setInputValue] = useState(value);
  const { debouncedQuery, suggestions } = useAddressSuggestionQuery(inputValue);

  // Программный фокус при открытии (поисковая шапка — каретка в поле).
  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  const rows = suggestions ?? [];
  const query = debouncedQuery.trim();
  const searched = query.length >= SUGGEST_QUERY_MIN_LENGTH;
  const showList = searched && rows.length > 0;
  const showNotFound = searched && suggestions !== undefined && rows.length === 0;
  const emptyText = inputValue.trim() === '' ? 'Введите адрес объекта' : 'Адрес не найден';
  const showEmptyState = inputValue.trim() === '' || showNotFound;

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Поиск адреса"
      className="fixed inset-0 z-50 overflow-y-auto bg-white"
    >
      <TopNav
        variant="search"
        leading={<IconButton icon={<Cancel />} label="Закрыть" onClick={onClose} />}
      >
        <SearchField
          ref={inputRef}
          placeholder="Найти адрес"
          aria-label="Найти адрес"
          value={inputValue}
          onChange={handleChange}
          onKeyDown={handleKeyDown}
          onClear={clear}
        />
      </TopNav>
      <PageContent className="pt-0">
        {showList && (
          <AddressSuggestionList
            containerRef={listRef}
            rowClassName="px-3 py-3"
            suggestions={rows}
            onSelect={onSelect}
            onEscape={refocusField}
          />
        )}
        {showEmptyState && (
          <div className="flex flex-col items-center gap-4 pt-16">
            <Image
              src="/images/properties/address-search-empty.png"
              alt=""
              width={128}
              height={128}
              className="h-32 w-32"
            />
            <p className="max-w-[280px] text-center text-base leading-[18px] text-content-secondary">
              {emptyText}
            </p>
          </div>
        )}
      </PageContent>
    </div>
  );

  function handleChange(event: ChangeEvent<HTMLInputElement>): void {
    setInputValue(event.currentTarget.value);
    onChange(event.currentTarget.value);
  }

  function handleKeyDown(event: KeyboardEvent<HTMLInputElement>): void {
    if (event.key === 'Escape') {
      onClose();
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
  // фокус возвращается в поле поиска.
  function refocusField(): void {
    inputRef.current?.focus();
  }

  function clear(): void {
    setInputValue('');
    onChange('');
    inputRef.current?.focus();
  }
}
