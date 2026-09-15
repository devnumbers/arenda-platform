'use client';

import type { JSX, KeyboardEvent, RefObject } from 'react';
import { addressSuggestionRow } from '@/features/properties';
import { ListRow } from '@/shared/ui/design';

/**
 * Список подсказок адреса DaData — общая часть шага «Адрес» визарда
 * создания и поиска адреса формы правки (карта #583, тикет #590).
 * Контейнер role=listbox (фокусируется программно, Tab-порядок не
 * занимает — в список входят стрелками с поля через focusListboxEdge),
 * строки role=option с roving focus; Escape на строке всплывает и
 * передаётся вызывающему (список гасит тот, кто им владеет).
 */
export type AddressSuggestionListProps = {
  readonly suggestions: ReadonlyArray<{ readonly value: string; readonly city?: string }>;
  readonly onSelect: (address: string) => void;
  /** Escape на контейнере/строке: спрятать список, вернуть фокус в поле. */
  readonly onEscape: () => void;
  /** Ref контейнера — вызывающий ведёт стрелками с поля. */
  readonly containerRef: RefObject<HTMLDivElement | null>;
  readonly containerClassName?: string;
  readonly rowClassName?: string;
};

export function AddressSuggestionList({
  suggestions,
  onSelect,
  onEscape,
  containerRef,
  containerClassName,
  rowClassName,
}: AddressSuggestionListProps): JSX.Element {
  const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>): void => {
    if (event.key === 'Escape') {
      onEscape();
    }
  };

  return (
    <div
      ref={containerRef}
      role="listbox"
      aria-label="Подсказки адреса"
      // Контейнер фокусируем программно (требование ARIA-listbox),
      // Tab-порядок не занимает: в список входят стрелками с поля.
      tabIndex={-1}
      className={containerClassName}
      onKeyDown={handleKeyDown}
    >
      {suggestions.map((suggestion) => {
        const row = addressSuggestionRow(suggestion.value, suggestion.city);
        return (
          <ListRow
            key={suggestion.value}
            className={rowClassName}
            subtitleClassName="text-content-secondary"
            title={row.title}
            subtitle={row.subtitle}
            variant="option"
            onSelect={() => onSelect(suggestion.value)}
          />
        );
      })}
    </div>
  );
}
