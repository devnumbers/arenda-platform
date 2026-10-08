'use client';

import type { JSX } from 'react';
import { RadioFalse, RadioTrue } from '@/shared/assets/icons';
import { ListRow } from '@/shared/ui/design';
import { PAYMENT_REMINDER_OPTIONS } from '../lib/reminder-offsets';
import type { PaymentReminderOffset } from '../model/types';

/**
 * Выбор напоминания о платеже «за N дней» (макеты шага 4 3214-72559/72379,
 * раньше 1084-24863): четыре радио-строки «Не напоминать / За 1 день /
 * За 3 дня / За 7 дней» на каноне Row Button + RadioFalse/RadioTrue (как
 * шаг «Категория платежа», Figma 781:12299). «Не напоминать» — первый
 * пункт и значение по умолчанию (#1193): value undefined — он и выбран;
 * явный тап возвращает с «за N дней» к отсутствию напоминаний (в команде
 * создания поле опускается). Выбранный пункт «за N дней» не снимается
 * повторным тапом (радио). Экран настроек аренды (#826) по своему макету
 * 1428-58757 взял селект (PickerSelectField из widgets/rentals/ui/
 * wizard-chrome), общий с этим пикером — кортеж опций
 * PAYMENT_REMINDER_OPTIONS.
 */

export type PaymentReminderPickerProps = {
  /** Выбранный оффсет; undefined — «Не напоминать» (дефолт, #1193). */
  readonly value: PaymentReminderOffset | undefined;
  readonly onChange: (offset: PaymentReminderOffset | undefined) => void;
};

export function PaymentReminderPicker({
  value,
  onChange,
}: PaymentReminderPickerProps): JSX.Element {
  return (
    <div className="flex flex-col pt-4">
      <ListRow
        title="Не напоминать"
        className="py-3.5"
        onSelect={() => onChange(undefined)}
        trailing={
          value === undefined ? (
            <RadioTrue className="h-6 w-6" aria-hidden />
          ) : (
            <RadioFalse className="h-6 w-6" aria-hidden />
          )
        }
      />
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
