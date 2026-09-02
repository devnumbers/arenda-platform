'use client';

import { useId } from 'react';
import type { ComponentProps, JSX } from 'react';
import { cn } from '@/shared/lib/cn';

/** Многострочное поле дизайн-слоя (тикет #505, «Заметка» формы контакта,
 * Figma 1281:48439): shell как у TextField (titleOut) — заголовок R/500 16/18
 * над боксом, строка подписи 13/15 под ним, — но бокс 92px (паддинг 18/10,
 * четыре строки 16/18 с прокруткой сверх), текст выровнен по верху, без
 * кнопки очистки. Состояния — как у TextField: Error (красный бокс + текст
 * ошибки), счётчик «длина/лимит» красным при достижении лимита (лимит формы
 * контакта — 1024), Disabled (opacity 0.5), Hover (inset-обводка 2px);
 * фокус-кольца нет намеренно (решение владельца 2026-08-26) — видимый
 * признак фокуса, каретка. */

export type TextareaProps = Omit<ComponentProps<'textarea'>, 'size'> & {
  readonly title?: string;
  readonly description?: string;
  readonly error?: string;
  /** Лимит символов: нативный maxLength текстовой области (ввод сверх
   * запрещён) + счётчик «длина/лимит»; красным — при достижении лимита. */
  readonly maxLength?: number;
};

export function Textarea({
  className,
  title,
  description,
  error,
  maxLength,
  value,
  disabled,
  ...props
}: TextareaProps): JSX.Element {
  const textareaId = useId();
  const counter =
    maxLength !== undefined && typeof value === 'string'
      ? `${value.length}/${maxLength}`
      : undefined;
  const counterDanger = maxLength !== undefined && typeof value === 'string' && value.length >= maxLength;
  const bottomLeft = error ?? description;

  return (
    <div className={cn('flex w-full flex-col gap-2 font-sans', disabled && 'opacity-50', className)}>
      {title !== undefined && (
        <label htmlFor={textareaId} className="text-base font-medium leading-[18px] text-content">
          {title}
        </label>
      )}
      <textarea
        id={textareaId}
        className={cn(
          'w-full resize-none rounded-button bg-surface-muted px-[18px] py-[10px] text-base leading-[18px] text-content outline-none transition-shadow placeholder:text-content-secondary',
          'min-h-[92px]',
          !disabled && error === undefined && 'hover:shadow-[inset_0_0_0_2px_var(--dl-input-border)]',
          error !== undefined && 'bg-surface-danger hover:shadow-none',
        )}
        disabled={disabled}
        value={value}
        maxLength={maxLength}
        {...props}
      />
      {(bottomLeft !== undefined || counter !== undefined) && (
        <div className="flex items-center justify-between gap-2 text-[13px] leading-[15px]">
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
