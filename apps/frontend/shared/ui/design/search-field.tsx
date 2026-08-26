'use client';

import type { ComponentProps, JSX } from 'react';
import { Cancel, Search } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';
import { IconButton } from './icon-button';

/** Поисковое поле дизайн-слоя (Figma 706:12561, выверено рендером 1:1):
 * пилюля 44px radius 100 на фоне #F3F4F6, текст M/400 14/16, плейсхолдер
 * #9FA8AC; слева паддинг 18, справа зона иконки 44×44 — в покое лупа,
 * при непустом значении и onClear — крестик (IconButton secondary,
 * #9FA8AC, остаётся в таб-порядке). Нативный крестик input[type=search]
 * (WebKit) скрыт — иначе их два. Фокус-кольца нет намеренно (решение
 * владельца 2026-08-26): видимый признак фокуса — каретка. */
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
        'flex h-11 w-full items-center rounded-pill bg-surface-muted pl-[18px] font-sans',
        disabled && 'opacity-50',
        className,
      )}
    >
      <input
        type="search"
        className="min-w-0 flex-1 border-none bg-transparent text-sm leading-4 text-content outline-none placeholder:text-content-tertiary [&::-webkit-search-cancel-button]:hidden"
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
          disabled={disabled}
          onClick={onClear}
        />
      ) : (
        <span
          className="flex h-11 w-11 shrink-0 items-center justify-center text-content-tertiary"
          aria-hidden
        >
          <Search className="h-6 w-6" />
        </span>
      )}
    </div>
  );
}
