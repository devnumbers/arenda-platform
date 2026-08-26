'use client';

import { useId } from 'react';
import type { ComponentProps, JSX } from 'react';
import { Cancel } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';
import { IconButton } from './icon-button';

/** Поле ввода дизайн-слоя (Figma 948:46646, «Input Field»): бокс фиксированной
 * высоты 56px в обоих вариантах. Title Out — заголовок над боксом, строка по
 * центру. Title In — плавающий лейбл: в покое плейсхолдер (16px) лежит на
 * строке ввода, при фокусе или непустом значении уменьшается до 13px и
 * уезжает наверх (раскладка строки: 10px + лейбл 15 + 2 + значение 18).
 * Состояния: Error (красный бокс + текст ошибки), Limited (счётчик красным),
 * Disabled (opacity 0.5), Hover (inset-обводка 2px) — фокус-кольца у поля
 * нет намеренно (решение владельца 2026-08-26): видимый признак фокуса —
 * каретка. Кнопка очистки появляется при непустом значении и переданном
 * onClear; остаётся в таб-порядке. */

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

/** Показатели счётчика и ошибок — кегль Figma Mobile/Text/S (13/15). */

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
  placeholder,
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
    'flex h-14 w-full items-center rounded-button bg-surface-muted pl-[18px] pr-1 transition-shadow',
    !disabled && error === undefined && 'hover:shadow-[inset_0_0_0_2px_var(--dl-input-border)]',
    error !== undefined && 'bg-surface-danger hover:shadow-none',
  );
  const input = cn(
    'h-full w-full min-w-0 border-none bg-transparent text-base leading-[18px] text-content outline-none placeholder:text-content-secondary',
  );

  return (
    <div className={cn('flex w-full flex-col gap-2 font-sans', disabled && 'opacity-50', className)}>
      {variant === 'titleOut' && title !== undefined && (
        <label htmlFor={inputId} className="text-base font-medium leading-[18px] text-content">
          {title}
        </label>
      )}
      <div className={box}>
        {variant === 'titleIn' && title !== undefined && (
          <div className="relative h-full min-w-0 flex-1">
            {/* Инпут идёт перед лейблом: peer-варианты требуют, чтобы peer
                предшествовал цели (~). Лейбл — плавающий: в покое крупный
                «плейсхолдер» на строке ввода; при фокусе или значении —
                мелкий, наверху. Плейсхолдер самого инпута прозрачен: его
                роль играет лейбл. */}
            <input
              id={inputId}
              className="peer h-full w-full border-none bg-transparent pt-[27px] text-base leading-[18px] text-content outline-none placeholder:text-transparent"
              disabled={disabled}
              value={value}
              placeholder={title}
              {...props}
            />
            <label
              htmlFor={inputId}
              className={cn(
                'pointer-events-none absolute left-0 text-content-secondary transition-all duration-200',
                'top-[27px] text-base leading-[18px]',
                'peer-focus:top-[10px] peer-focus:text-[13px] peer-focus:leading-[15px]',
                'peer-[:not(:placeholder-shown)]:top-[10px] peer-[:not(:placeholder-shown)]:text-[13px] peer-[:not(:placeholder-shown)]:leading-[15px]',
              )}
            >
              {title}
            </label>
          </div>
        )}
        {(variant === 'titleOut' || title === undefined) && (
          <input id={inputId} className={input} disabled={disabled} value={value} placeholder={placeholder} {...props} />
        )}
        {showClear && (
          <IconButton
            icon={<Cancel />}
            label="Очистить поле"
            variant={error !== undefined ? 'danger' : 'secondary'}
            className="h-10 w-10 shrink-0"
            disabled={disabled}
            onClick={onClear}
          />
        )}
      </div>
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
