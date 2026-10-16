'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import { RadioFalse, RadioTrue, SmallArrowRight } from '@/shared/assets/icons';
import type { IsoDate, PaymentReminderOffset, Recurrence } from '@/entities/payment';
import { formatDayMonthWithYear, PaymentReminderPicker } from '@/entities/payment';
import { useMe } from '@/features/auth';
import {
  emailReminderCaption,
  EmailNotificationsRow,
} from '@/features/notifications';
import {
  periodicityReady,
  type PaymentDraftType,
} from '@/features/payments';
import { CalendarDatePicker, ListRow, Switch } from '@/shared/ui/design';
import { firstOccurrencePreview } from '../../lib/first-occurrence';
import { WizardHeading } from './wizard-chrome';

/**
 * Шаг 4 визарда — напоминания/уведомления по макетам 3214-72559/72379
 * (ручной платёж) и 3214-72739 (автоплатёж), тикет #1193; обе ветки несут
 * H1-заголовки канона #1152 и общую секцию «Настройки»:
 *
 * — ручной платёж: «За сколько напомнить об оплате» с радио «Не
 *   напоминать» (по умолчанию, #1193) / «За 1 / 3 / 7 дней» (контракт
 *   reminderOffsetDays, карта #822);
 * — автоплатёж: «Уведомлять об оплате» с радио «Не уведомлять» (дефолт) /
 *   «Да, уведомлять» — пер-платёжный флаг notifyAutoPaid (#1189); радио
 *   напоминаний у автоплатежа нет (сервер при создании форсирует оффсет
 *   в null, решение гриллинга #1186).
 *
 * Секция «Настройки» (без подзаголовка, по макету): тумблер «Уведомления
 * на почту» — общий EmailNotificationsRow (шоткат глобальной email-настройки
 * категории «Платежи и операции», канон #746; опечатки подписей макетов не
 * воспроизводятся, канон AutoPayRow) и тумблер «Окончание платежа»
 * («Необязательно»): включённым открывает строку «Выбрать дату» (канон
 * Figma 843:8345), выключенным чистит выбранную дату.
 *
 * Окончание (#1155): в пикере endDate дни раньше первого вхождения
 * расписания (since = «сегодня» владельца — сервер ставит его сам,
 * ADR 0048) недоступны — бекенд-инвариант «окна графика» (#1150)
 * профилактируется на фронте; заголовок пикера — название поля, канон
 * аренды («Окончание аренды»). Стоящее окончание при смене периодичности
 * чистит черновик визарда (draftAfterRecurrenceChange, решение владельца
 * 2026-10-06).
 */

export type PaymentSettingsStepProps = {
  readonly draftType: PaymentDraftType;
  readonly reminderOffsetDays: PaymentReminderOffset | undefined;
  readonly onReminderOffsetDaysChange: (offset: PaymentReminderOffset | undefined) => void;
  /** Флаг «уведомлять об автоплатеже»; undefined — «Не уведомлять» (дефолт). */
  readonly notifyAutoPaid: boolean | undefined;
  readonly onNotifyAutoPaidChange: (notifyAutoPaid: boolean | undefined) => void;
  readonly endDate: IsoDate | undefined;
  readonly onEndDateChange: (endDate: IsoDate | undefined) => void;
  readonly recurrence: Recurrence | undefined;
  readonly today: IsoDate;
};

export function PaymentSettingsStep({
  draftType,
  reminderOffsetDays,
  onReminderOffsetDaysChange,
  notifyAutoPaid,
  onNotifyAutoPaidChange,
  endDate,
  onEndDateChange,
  recurrence,
  today,
}: PaymentSettingsStepProps): JSX.Element {
  const [pickerOpen, setPickerOpen] = useState(false);
  // Тумблер — локальное состояние раскрытия строки даты: включённым
  // открывает «Выбрать дату», выключенным чистит дату (сама дата —
  // единственное, что едет в черновик).
  const [endDateOn, setEndDateOn] = useState(endDate !== undefined);
  const meQuery = useMe();
  const email = meQuery.data?.email ?? null;
  // Минимум пикера окончания — первое вхождение готового расписания
  // (окончание в расчёт не берётся, иначе наивное окно «съедает» всё).
  const endDateMinDate =
    recurrence !== undefined && periodicityReady(recurrence)
      ? firstOccurrencePreview(recurrence, today) ?? undefined
      : undefined;

  const toggleEndDate = (checked: boolean): void => {
    setEndDateOn(checked);
    if (!checked) {
      onEndDateChange(undefined);
    }
  };

  return (
    <>
      {draftType === 'payment' ? (
        <>
          <WizardHeading
            variant="h1"
            title="За сколько напомнить об оплате"
            subtitle="Пришлем в 10:00 напоминание об оплате"
          />
          <PaymentReminderPicker
            value={reminderOffsetDays}
            onChange={onReminderOffsetDaysChange}
          />
        </>
      ) : (
        <>
          <WizardHeading
            variant="h1"
            title="Уведомлять об оплате"
            subtitle="Будем отмечать оплату и в 10:00 присылать уведомление, что платеж оплачен"
          />
          <NotifyAutoPaidPicker value={notifyAutoPaid} onChange={onNotifyAutoPaidChange} />
        </>
      )}

      {/* 16px секционного зазора макета + 24px собственных отступов
          заголовка — итого 40px от последней радио-строки. */}
      <StepSectionHeading title="Настройки" />
      <div className="flex flex-col pt-4">
        <EmailNotificationsRow title="Уведомления на почту" caption={emailReminderCaption('о платеже', email)} />
        <EndDateToggleRow checked={endDateOn} onToggle={toggleEndDate} />
        {endDateOn && (
          <EndDateRow
            endDate={endDate}
            today={today}
            onOpen={() => setPickerOpen(true)}
          />
        )}
      </div>

      {/* Рендер только в открытом состоянии — состояние ленты и черновик
          живут, пока пикер смонтирован (конвенция канона). */}
      {pickerOpen && (
        <CalendarDatePicker
          title="Окончание платежа"
          today={today}
          value={endDate ?? null}
          minDate={endDateMinDate}
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

/** Секционный заголовок «Настройки» под главным (H1-кегль 28/32 макета
 * 3214:72992, но h2 — на шаге один h1). */
function StepSectionHeading({ title }: { readonly title: string }): JSX.Element {
  return (
    <div className="px-6 pt-10">
      <h2 className="m-0 text-2xl font-semibold leading-8 text-content">{title}</h2>
    </div>
  );
}

/** Радио «уведомлять об автоплатеже» (макет 3214-72739): «Не уведомлять» —
 * значение по умолчанию (undefined, черновик поля не пишет), «Да,
 * уведомлять» — флаг notifyAutoPaid (#1189). */
function NotifyAutoPaidPicker({
  value,
  onChange,
}: {
  readonly value: boolean | undefined;
  readonly onChange: (notifyAutoPaid: boolean | undefined) => void;
}): JSX.Element {
  const options: ReadonlyArray<{
    readonly label: string;
    readonly checked: boolean;
    readonly pick: () => void;
  }> = [
    { label: 'Не уведомлять', checked: value !== true, pick: () => onChange(undefined) },
    { label: 'Да, уведомлять', checked: value === true, pick: () => onChange(true) },
  ];
  return (
    <div className="flex flex-col pt-4">
      {options.map((option) => (
        <ListRow
          key={option.label}
          title={option.label}
          className="py-3.5"
          onSelect={option.pick}
          trailing={
            option.checked ? (
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

/** Строка-тумблер «Окончание платежа» (макет 3214:72995, анатомия
 * EmailNotificationsRow): подпись «Необязательно», включённым раскрывает
 * строку даты. */
function EndDateToggleRow({
  checked,
  onToggle,
}: {
  readonly checked: boolean;
  readonly onToggle: (checked: boolean) => void;
}): JSX.Element {
  return (
    <div className="flex items-center justify-between gap-4 px-6 py-3">
      <div className="flex min-w-0 flex-col gap-1">
        <span className="text-base font-medium leading-[18px] text-content">Окончание платежа</span>
        <span className="text-sm leading-4 text-content-tertiary">Необязательно</span>
      </div>
      <Switch
        checked={checked}
        onCheckedChange={(value) => onToggle(value === true)}
        aria-label="Окончание платежа"
      />
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
    <ListRow
      title={endDate !== undefined ? formatDayMonthWithYear(endDate, today) : 'Выбрать дату'}
      className="py-4"
      onSelect={onOpen}
      trailing={<SmallArrowRight className="h-6 w-6" aria-hidden />}
    />
  );
}
