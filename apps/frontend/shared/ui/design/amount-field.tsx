'use client';

import type { ChangeEvent, JSX } from 'react';
import { cn } from '@/shared/lib/cn';
import { groupedAmount, sanitizeAmountInput } from './amount-input';

/** Поле суммы дизайн-слоя (тикет #459, Figma 834:19662 / 835:19795):
 * крупная центрированная строка «2 500 ₽» (Onest SemiBold 44/48) — правка
 * владельца 2026-08-26: ввод с обычной клавиатуры вместо numpad, только
 * цифры и запятая. Маска — sanitizeAmountInput (фильтрует символы, лимит
 * 9 999 999,99 ₽); на мобильной клавиатуре открывается цифровой блок
 * (inputMode=decimal), точка нормализуется в запятую. Значение — «сырая»
 * строка («1234,5»), в поле рисуется группировка разрядов. */
export type AmountFieldProps = {
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly disabled?: boolean;
  /** Имя поля для скринридеров («Сумма»). */
  readonly label: string;
  readonly className?: string;
};

export function AmountField({
  value,
  onChange,
  disabled = false,
  label,
  className,
}: AmountFieldProps): JSX.Element {
  // Управляемый input показывает маскированное значение с символом ₽;
  // при вводе маска вычищает лишнее — если DOM разошёлся со значением,
  // синхронизируем его вручную (React не перерисует совпавший value).
  const handleChange = (event: ChangeEvent<HTMLInputElement>): void => {
    const sanitized = sanitizeAmountInput(event.target.value);
    onChange(sanitized);
    if (event.target.value !== displayValue(sanitized)) {
      event.target.value = displayValue(sanitized);
    }
  };

  return (
    <input
      type="text"
      inputMode="decimal"
      autoComplete="off"
      spellCheck={false}
      pattern="[0-9]*"
      aria-label={label}
      disabled={disabled}
      value={displayValue(value)}
      placeholder="0 ₽"
      onChange={handleChange}
      className={cn(
        'w-full cursor-text bg-transparent text-center font-sans text-[2.75rem] font-semibold leading-12 text-content caret-primary outline-none transition-colors placeholder:text-content-tertiary',
        'focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface',
        'disabled:pointer-events-none disabled:opacity-50',
        className,
      )}
    />
  );
}

function displayValue(value: string): string {
  const grouped = groupedAmount(value);
  return grouped === '' ? '' : `${grouped}\u00A0₽`;
}
