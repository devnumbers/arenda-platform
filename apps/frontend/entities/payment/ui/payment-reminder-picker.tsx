'use client';

import type { JSX } from 'react';
import { RadioFalse, RadioTrue } from '@/shared/assets/icons';
import { ListRow } from '@/shared/ui/design';
import { PAYMENT_REMINDER_OPTIONS } from '../lib/reminder-offsets';
import type { PaymentReminderOffset } from '../model/types';

/**
 * Выбор напоминания о платеже «за N дней» (карта #822, макет 1084-24863):
 * три радио-строки «За 1 день / За 3 дня / За 7 дней» на каноне
 * Row Button + RadioFalse/RadioTrue (как шаг «Категория платежа»,
 * Figma 781:12299). Выбор опционален: начальное состояние — ничего
 * не выбрано; выбранный пункт не снимается повторным тапом (радио).
 * Экран настроек аренды (#826) по своему макету 1428-58757 взял селект
 * (ReminderPickerField), общий с этим пикером — кортеж опций
 * PAYMENT_REMINDER_OPTIONS.
 */

export type PaymentReminderPickerProps = {
  /** Выбранный оффал; undefined — ничего не выбрано (дефолт). */
  readonly value: PaymentReminderOffset | undefined;
  readonly onChange: (offset: PaymentReminderOffset) => void;
};

export function PaymentReminderPicker({
  value,
  onChange,
}: PaymentReminderPickerProps): JSX.Element {
  return (
    <div className="flex flex-col pt-2">
      {PAYMENT_REMINDER_OPTIONS.map((option) => (
        <ListRow
          key={option.offset}
          title={option.label}
          className="py-3.5"
          onSelect={() => onChange(option.offset)}
          trailing={
            option.offset === value ? (
              <RadioTrue className="h-6 w-6" aria-hidden />
            ) : (
              <RadioFalse className="h-6 w-6" aria-hidden />
            )
          }
        />
      ))}
    </div>
  );
}
