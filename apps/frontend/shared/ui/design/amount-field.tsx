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
 * строка («1234,5»), в поле рисуется группировка разрядов. Фокусной
 * обводки нет намеренно (в макете фокус не отрисован); позицию ввода
 * показывает каретка цвета текста.
 *
 * Символ «₽» — снаружи инпута: суффикс-элемент рядом с цифрами (правка
 * владельца 2026-08-26: будучи частью value, он уводил каретку за себя и
 * ломал Backspace). Сам инпут скрыт под невидимым span-измерителем той же
 * строки: ширина поля равна ширине набранного текста, связка центрируется,
 * каретка живёт среди цифр, Backspace стирает по символу. */
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
  // Управляемый input показывает маскированное значение; при вводе маска
  // вычищает лишнее — если DOM разошёлся со значением, синхронизируем его
  // вручную (React не перерисует совпавший value).
  const handleChange = (event: ChangeEvent<HTMLInputElement>): void => {
    const sanitized = sanitizeAmountInput(event.target.value);
    onChange(sanitized);
    const domValue = groupedAmount(sanitized);
    if (event.target.value !== domValue) {
      event.target.value = domValue;
    }
  };

  const amountStyle =
    'font-sans text-[2.75rem] font-semibold leading-12';

  return (
    <span
      className={cn(
        'inline-flex items-baseline justify-center',
        disabled && 'opacity-50',
        className,
      )}
    >
      <span className="relative inline-flex">
        {/* измеритель: задаёт ширину инпута по набранному тексту */}
        <span aria-hidden className={cn('invisible whitespace-pre px-0.5', amountStyle)}>
          {groupedAmount(value) === '' ? '0' : groupedAmount(value)}
        </span>
        <input
          type="text"
          inputMode="decimal"
          autoComplete="off"
          spellCheck={false}
          pattern="[0-9]*"
          size={1}
          aria-label={label}
          disabled={disabled}
          value={groupedAmount(value)}
          placeholder="0"
          onChange={handleChange}
          className={cn(
            'absolute inset-0 h-full w-full cursor-text bg-transparent text-center outline-none placeholder:text-content-tertiary',
            amountStyle,
            'text-content',
          )}
        />
      </span>
      <span
        aria-hidden
        className={cn(
          // ml-3 — пробел между суммой и символом рубля, как в макете «2 500 ₽»
          'ml-3',
          amountStyle,
          value === '' ? 'text-content-tertiary' : 'text-content',
        )}
      >
        ₽
      </span>
    </span>
  );
}
