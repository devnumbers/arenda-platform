'use client';

import type { JSX } from 'react';
import {
  PAYMENT_REMINDER_OPTIONS,
  paymentReminderOptionLabel,
  type PaymentReminderOffset,
} from '@/entities/payment';
import { useMe } from '@/features/auth';
import {
  emailReminderCaption,
  EmailNotificationsRow,
} from '@/features/notifications';
import { AutoPayRow, PickerSelectField, WizardHeading } from './wizard-chrome';

/**
 * Шаг «Настройки аренды» (карта #822, тикет #826; Figma 1428-58757):
 * тумблер автоплатежа (канон AutoPayRow), селект «За сколько напоминать»
 * и тумблер «Включить уведомления об оплате на почту» — общий
 * EmailNotificationsRow (шоткат категории «Платежи и операции»). Селект
 * виден независимо от тумблера (по макету оба блока на экране
 * одновременно); #1198 (решение владельца 07.10): опции «Не напоминать /
 * За 1 день / За 3 дня / За 7 дней», дефолт — «Не напоминать» (отсутствие
 * выбора), в команду уходит явный null. Выбранный оффсет протекает в
 * создаваемый арендой Платёж 1:1 (buildRentalCreateCommand). Подпись почты
 * повторяет структуру канона шага 4 платежей (#825) с арендным предметом
 * («об оплате») — сломанную грамматику макета не воспроизводим. Ранее
 * экран нёс один тумблер: email-тумблеры макета были вырезаны решением
 * картирования #526 — карта #822 решение отменяет.
 */

export type SettingsStepProps = {
  readonly autoPay: boolean;
  readonly onAutoPayChange: (autoPay: boolean) => void;
  /** Выбранный оффсет; undefined — «Не напоминать» (дефолт шага, #1198). */
  readonly reminderOffsetDays: PaymentReminderOffset | undefined;
  readonly onReminderOffsetChange: (offset: PaymentReminderOffset | undefined) => void;
};

export function SettingsStep({
  autoPay,
  onAutoPayChange,
  reminderOffsetDays,
  onReminderOffsetChange,
}: SettingsStepProps): JSX.Element {
  const meQuery = useMe();
  const email = meQuery.data?.email ?? null;
  const emailCaption = emailReminderCaption('об оплате', email);

  return (
    <>
      <WizardHeading title="Настройки аренды" />
      <div className="flex flex-col gap-8 px-6 pt-6">
        <AutoPayRow checked={autoPay} onCheckedChange={onAutoPayChange} />
        <PickerSelectField
          title="За сколько напоминать"
          valueLabel={
            reminderOffsetDays === undefined
              ? 'Не напоминать'
              : paymentReminderOptionLabel(reminderOffsetDays)
          }
          groups={[
            {
              options: [
                {
                  label: 'Не напоминать',
                  selected: reminderOffsetDays === undefined,
                  onSelect: () => onReminderOffsetChange(undefined),
                },
                ...PAYMENT_REMINDER_OPTIONS.map((option) => ({
                  label: option.label,
                  selected: reminderOffsetDays === option.offset,
                  onSelect: () => onReminderOffsetChange(option.offset),
                })),
              ],
            },
          ]}
        />
      </div>
      {/* py-3 строки + pt-5 — те же 32px до тумблера почты, что между
          блоками в колонке (макет 1428-58757). */}
      <div className="pt-5">
        <EmailNotificationsRow
          title="Включить уведомления об оплате на почту"
          caption={emailCaption}
        />
      </div>
    </>
  );
}
