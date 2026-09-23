'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import { SmallArrowRight } from '@/shared/assets/icons';
import type { IsoDate, PaymentReminderOffset } from '@/entities/payment';
import { formatDayMonthWithYear, PaymentReminderPicker } from '@/entities/payment';
import { useMe } from '@/features/auth';
import { EmailNotificationsRow } from '@/features/notifications';
import type { PaymentDraftType } from '@/features/payments';
import { CalendarDatePicker, ListRow } from '@/shared/ui/design';
import { WizardHeading } from './wizard-chrome';

/**
 * Шаг 4 визарда — настройки платежа, две ветки по типу создания
 * (решение постановки #823, тикет #825):
 *
 * — ручной платёж (Figma 1084-24863): «Настройте платеж» с радио
 *   «За 1 день / За 3 дня / За 7 дней» (опционально, по умолчанию ничего
 *   не выбрано; контракт reminderOffsetDays) и секция «Настройки платежа»
 *   — окончание и почта;
 * — автоплатёж (Figma 1056-54338): без радио — карточка «Уведомления
 *   об оплате» (автоплатёж уведомит, когда платёж отметят оплаченным),
 *   «Окончание платежа» остаётся с каноническим заголовком.
 *
 * Тумблер «Уведомления на почту» — общий EmailNotificationsRow (шоткат
 * глобальной email-настройки категории «Платежи и операции», канон
 * #746); опечатки подписей макетов не воспроизводятся (канон AutoPayRow).
 */

export type PaymentSettingsStepProps = {
  readonly draftType: PaymentDraftType;
  readonly reminderOffsetDays: PaymentReminderOffset | undefined;
  readonly onReminderOffsetDaysChange: (offset: PaymentReminderOffset) => void;
  readonly endDate: IsoDate | undefined;
  readonly onEndDateChange: (endDate: IsoDate | undefined) => void;
  readonly today: IsoDate;
};

export function PaymentSettingsStep({
  draftType,
  reminderOffsetDays,
  onReminderOffsetDaysChange,
  endDate,
  onEndDateChange,
  today,
}: PaymentSettingsStepProps): JSX.Element {
  const [pickerOpen, setPickerOpen] = useState(false);
  const meQuery = useMe();
  const email = meQuery.data?.email ?? null;
  const emailCaption =
    email !== null
      ? `Будем напоминать о платеже на вашу почту ${email}`
      : 'Будем напоминать о платеже на вашу почту';

  return (
    <>
      {draftType === 'payment' ? (
        <>
          <WizardHeading
            title="Настройте платеж"
            subtitle="Выберите за сколько дней напомнить о платеже. В нужный день пришлем напоминание о том, что платеж нужно отметить"
          />
          <PaymentReminderPicker
            value={reminderOffsetDays}
            onChange={onReminderOffsetDaysChange}
          />
          <StepSectionHeading
            title="Настройки платежа"
            subtitle="Можно выбрать окончание платежа и добавить напоминания об оплате на электронную почту"
          />
          <EndDateRow
            endDate={endDate}
            today={today}
            onOpen={() => setPickerOpen(true)}
          />
          <EmailNotificationsRow title="Уведомления на почту" caption={emailCaption} />
        </>
      ) : (
        <>
          <div className="mx-6 mt-6 flex flex-col gap-2 rounded-3xl bg-surface-muted px-6 pb-6 pt-6">
            <h2 className="m-0 text-xl font-semibold leading-6 text-content">
              Уведомления об оплате
            </h2>
            <p className="text-sm leading-4 text-content-secondary">
              Пришлём уведомление, когда отметим платеж оплаченным
            </p>
          </div>
          <WizardHeading
            title="Окончание платежа"
            subtitle="После выбранной даты, платеж перестанет оплачиваться и удалится. Выбирать необязательно"
          />
          <EndDateRow
            endDate={endDate}
            today={today}
            onOpen={() => setPickerOpen(true)}
          />
          <StepSectionHeading title="Уведомления на почту" subtitle={emailCaption} />
          <EmailNotificationsRow title="Уведомления на почту" caption={undefined} />
        </>
      )}

      {/* Рендер только в открытом состоянии — состояние ленты и черновик
          живут, пока пикер смонтирован (конвенция канона). */}
      {pickerOpen && (
        <CalendarDatePicker
          today={today}
          value={endDate ?? null}
          onClose={() => setPickerOpen(false)}
          onConfirm={(date) => {
            onEndDateChange(date ?? undefined);
            setPickerOpen(false);
          }}
        />
      )}
    </>
  );
}

/** Секционный заголовок шага под главным (H3 20/24 + подзаголовок 14/16,
 * как WizardHeading, но h2 — на шаге один h1). */
function StepSectionHeading({
  title,
  subtitle,
}: {
  readonly title: string;
  readonly subtitle?: string;
}): JSX.Element {
  return (
    <div className="flex flex-col gap-2 px-6 pt-6">
      <h2 className="m-0 text-xl font-semibold leading-6 text-content">{title}</h2>
      {subtitle !== undefined && (
        <p className="text-sm leading-4 text-content-secondary">{subtitle}</p>
      )}
    </div>
  );
}

/** Строка «Выбрать дату» (канон шага «Окончание платежа», Figma 843:8345):
 * с датой — она сама, тап открывает канонический бесконечный календарь. */
function EndDateRow({
  endDate,
  today,
  onOpen,
}: {
  readonly endDate: IsoDate | undefined;
  readonly today: IsoDate;
  readonly onOpen: () => void;
}): JSX.Element {
  return (
    <div className="flex flex-col pt-2">
      <ListRow
        title={endDate !== undefined ? formatDayMonthWithYear(endDate, today) : 'Выбрать дату'}
        className="py-4"
        onSelect={onOpen}
        trailing={<SmallArrowRight className="h-6 w-6" aria-hidden />}
      />
    </div>
  );
}
