'use client';

import type { JSX } from 'react';
import type { PaymentReminderOffset } from '@/entities/payment';
import { useMe } from '@/features/auth';
import {
  emailReminderCaption,
  NotifyEmailRow,
  RENTAL_EMAIL_CATEGORY,
} from '@/features/notifications';
import { AutoPayRow, ReminderPickerField, WizardHeading } from './wizard-chrome';

/**
 * Шаг «Настройки аренды» (карта #822, тикет #826; Figma 1428-58757):
 * тумблер автоплатежа (канон AutoPayRow), селект «За сколько напоминать»
 * и ряд «Включить уведомления об оплате на почту» — NotifyEmailRow,
 * персональная галочка почты этой аренды (карта #1551, тикет #1557;
 * разворот шотката #746 на арендном экране): значение живёт в черновике
 * (старт on, канон #1552), в команду создания едет флагом notifyEmail;
 * глобальный мастер категории «Аренда» решает видимость ряда вовсе.
 * Шоткат-запись матрицы с экрана снята — матрицу пишет только экран
 * настроек профиля. Гейта от пикера напоминаний у аренды нет — ряд
 * независим от него (решение владельца 22.10.2026, канон #1552), гасится
 * только мастером. Селект виден независимо от тумблера (по макету оба
 * блока на экране одновременно); #1198 (решение владельца 07.10): опции
 * «Не напоминать / За 1 день / За 3 дня / За 7 дней», дефолт — «Не
 * напоминать» (отсутствие выбора), в команду уходит явный null. Выбранный
 * оффсет протекает в создаваемый арендой Платёж 1:1
 * (buildRentalCreateCommand). Подпись почты повторяет структуру канона
 * шага 4 платежей (#825) с арендным предметом («об оплате») — сломанную
 * грамматику макета не воспроизводим. Ранее экран нёс один тумблер:
 * email-тумблеры макета были вырезаны решением картирования #526 — карта
 * #822 решение отменяет.
 */

export type SettingsStepProps = {
  readonly autoPay: boolean;
  readonly onAutoPayChange: (autoPay: boolean) => void;
  /** Персональная галочка почты аренды (карта #1551); не тронуто — on. */
  readonly notifyEmail: boolean;
  readonly onNotifyEmailChange: (notifyEmail: boolean) => void;
  /** Выбранный оффсет; undefined — «Не напоминать» (дефолт шага, #1198). */
  readonly reminderOffsetDays: PaymentReminderOffset | undefined;
  readonly onReminderOffsetChange: (offset: PaymentReminderOffset | undefined) => void;
};

export function SettingsStep({
  autoPay,
  onAutoPayChange,
  notifyEmail,
  onNotifyEmailChange,
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
        <ReminderPickerField value={reminderOffsetDays} onChange={onReminderOffsetChange} />
      </div>
      {/* py-3 строки + pt-5 — те же 32px до ряда почты, что между
          блоками в колонке (макет 1428-58757); при мастере off ряд
          схлопывается целиком — обёртка последняя на шаге, хвостовой
          зазор не виден. */}
      <div className="pt-5">
        <NotifyEmailRow
          category={RENTAL_EMAIL_CATEGORY}
          title="Включить уведомления об оплате на почту"
          caption={emailCaption}
          checked={notifyEmail}
          onCheckedChange={onNotifyEmailChange}
        />
      </div>
    </>
  );
}
