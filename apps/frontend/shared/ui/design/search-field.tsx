'use client';

import type { ComponentProps, JSX } from 'react';
import { Cancel, Search } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';
import { IconButton } from './icon-button';

/** Поисковое поле дизайн-слоя (Figma 706:12561): пилюля radius 100 на сером
 * фоне, текст 14/16, иконка поиска справа; при непустом значении и onClear —
 * кнопка-крестик (остаётся в таб-порядке). Фокус — каноничный ring
 * дизайн-системы (focus-within). */
export type SearchFieldProps = Omit<ComponentProps<'input'>, 'type' | 'size'> & {
  readonly onClear?: () => void;
};

export function SearchField({
  className,
  placeholder,
  value,
  disabled,
  onClear,
  ...props
}: SearchFieldProps): JSX.Element {
  const hasValue = typeof value === 'string' && value.length > 0;
  const showClear = onClear !== undefined && hasValue && !disabled;

  return (
    <div
      className={cn(
        'flex h-13 w-full items-center gap-2 rounded-pill bg-surface-muted pl-[18px] pr-1 font-sans transition-shadow',
        'focus-within:ring-4 focus-within:ring-primary focus-within:ring-offset-2 focus-within:ring-offset-surface',
        disabled && 'opacity-50',
        className,
      )}
    >
      <input
        type="search"
        className="min-w-0 flex-1 bg-transparent text-sm text-content outline-none placeholder:text-content-tertiary"
        placeholder={placeholder}
        disabled={disabled}
        value={value}
        {...props}
      />
      {showClear ? (
        <IconButton
          icon={<Cancel />}
          label="Очистить поиск"
          variant="secondary"
          className="h-10 w-10"
          disabled={disabled}
          onClick={onClear}
        />
      ) : (
        <span className="flex h-10 w-10 shrink-0 items-center justify-center text-content-secondary" aria-hidden>
          <Search className="h-6 w-6" />
        </span>
      )}
    </div>
  );
}
