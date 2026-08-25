'use client';

import { useId } from 'react';
import type { ComponentProps, JSX } from 'react';
import { Cancel } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';
import { IconButton } from './icon-button';

/** Поле ввода дизайн-слоя (Figma 948:46646): варианты Title Out (заголовок
 * над серым боксом) и Title In (заголовок внутри), состояния Error (красный
 * бокс + текст ошибки) и Limited (счётчик знаков красным). Кнопка очистки
 * появляется при непустом значении и переданном onClear; остаётся в таб-порядке.
 * Фокус с клавиатуры — каноничный ring дизайн-системы (focus-within), как у
 * SearchField: у Figma-компонента фокус-состояния нет, каретка одна не
 * проходит по а11y. */

export type TextFieldVariant = 'titleOut' | 'titleIn';

export type TextFieldProps = Omit<ComponentProps<'input'>, 'size'> & {
  readonly variant?: TextFieldVariant;
  readonly title?: string;
  readonly description?: string;
  readonly error?: string;
  /** Показывает счётчик «длина/лимит»; красным — при достижении лимита. */
  readonly maxLength?: number;
  readonly onClear?: () => void;
};

export function TextField({
  className,
  variant = 'titleOut',
  title,
  description,
  error,
  maxLength,
  onClear,
  value,
  disabled,
  ...props
}: TextFieldProps): JSX.Element {
  const inputId = useId();
  const hasValue = typeof value === 'string' && value.length > 0;
  const showClear = onClear !== undefined && hasValue && !disabled;
  const counter =
    maxLength !== undefined && typeof value === 'string'
      ? `${value.length}/${maxLength}`
      : undefined;
  const counterDanger = maxLength !== undefined && typeof value === 'string' && value.length >= maxLength;
  const bottomLeft = error ?? description;

  const box = cn(
    'flex w-full items-stretch rounded-button bg-surface-muted pl-[18px] pr-1 transition-shadow',
    !disabled && 'hover:shadow-[inset_0_0_0_2px_var(--dl-input-border)]',
    'focus-within:ring-4 focus-within:ring-primary focus-within:ring-offset-2 focus-within:ring-offset-surface',
    error !== undefined && 'bg-surface-danger hover:shadow-none',
  );
  const input = cn(
    'w-full min-w-0 border-none bg-transparent text-base text-content outline-none placeholder:text-content-secondary',
    variant === 'titleOut' && 'h-[38px]',
  );

  return (
    <div className={cn('flex w-full flex-col gap-2 font-sans', disabled && 'opacity-50', className)}>
      {variant === 'titleOut' && title !== undefined && (
        <label htmlFor={inputId} className="text-base font-medium text-content">
          {title}
        </label>
      )}
      <div className={box}>
        {variant === 'titleIn' && (
          <div className="flex min-w-0 flex-1 flex-col justify-center gap-0.5 py-2.5">
            {title !== undefined && (
              <label htmlFor={inputId} className="text-xs text-content-secondary">
                {title}
              </label>
            )}
            <input id={inputId} className={input} disabled={disabled} value={value} {...props} />
          </div>
        )}
        {variant === 'titleOut' && (
          <input id={inputId} className={input} disabled={disabled} value={value} {...props} />
        )}
        {showClear && (
          <IconButton
            icon={<Cancel />}
            label="Очистить поле"
            variant={error !== undefined ? 'danger' : 'secondary'}
            className="my-0.5 h-9 w-9 self-center"
            disabled={disabled}
            onClick={onClear}
          />
        )}
      </div>
      {(bottomLeft !== undefined || counter !== undefined) && (
        <div className="flex items-center justify-between gap-2 text-xs">
          {bottomLeft !== undefined && (
            <span className={cn(error !== undefined ? 'text-error' : 'text-content-tertiary')}>
              {bottomLeft}
            </span>
          )}
          {counter !== undefined && (
            <span className={cn(counterDanger ? 'text-error' : 'text-content-tertiary')}>{counter}</span>
          )}
        </div>
      )}
    </div>
  );
}
