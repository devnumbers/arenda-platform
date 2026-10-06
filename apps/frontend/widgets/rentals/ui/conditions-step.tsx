'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import { Calendar } from '@/shared/assets/icons';
import { type IsoDate } from '@/shared/lib/calendar';
import { formatDayMonthWithYear } from '@/shared/lib/date-format';
import { kopecksToAmountInputString, parseRublesToKopecks } from '@/shared/lib/format-money';
import type { RentalPaymentDay, RentalUtilities } from '@/entities/rental';
import {
  plannedEndDateMinDate,
  rentalPlannedEndDateError,
  rentalStartDateError,
} from '@/features/rentals';
import { CalendarDatePicker } from '@/shared/ui/design';
import {
  MoneyField,
  PickerTriggerBox,
  UtilitiesPickerField,
  WizardHeading,
} from './wizard-chrome';

/**
 * Шаг 2 «Условия аренды» (Figma 1270:46821/47960, шит даты 1270:37644):
 * начало* и опциональное окончание открывают канонический бесконечный
 * календарь CalendarDatePicker — окончание позже начала: дни ≤ начала в
 * его пикере погашены (minDate, решение владельца 2026-09-05, ADR 0053),
 * и не раньше первого вхождения дня оплаты (#1156): дата до первой
 * оплаты оставила бы аренду без платежей — минимум пикера ведёт первое
 * вхождение, когда оно позже дня после начала. Коммунальные платежи —
 * PickerMenu (радио-меню макета 1296:48965: «Включены в стоимость» /
 * «Только счетчики» / «Вся квитанция»), залог и комиссия — необязательные
 * денежные поля. Дата в поле — канонический формат «10 мая, 2027».
 */

export type ConditionsStepProps = {
  readonly startDate: IsoDate | undefined;
  readonly onStartDateChange: (startDate: IsoDate | undefined) => void;
  /** День оплаты шага 1 — известен к моменту выбора окончания (#1156). */
  readonly paymentDay: RentalPaymentDay | undefined;
  readonly plannedEndDate: IsoDate | undefined;
  readonly onPlannedEndDateChange: (plannedEndDate: IsoDate | undefined) => void;
  readonly utilities: RentalUtilities | undefined;
  readonly onUtilitiesChange: (utilities: RentalUtilities) => void;
  /** Копейки; undefined — не заданы. */
  readonly depositKopecks: number | undefined;
  readonly onDepositChange: (depositKopecks: number | undefined) => void;
  readonly commissionKopecks: number | undefined;
  readonly onCommissionChange: (commissionKopecks: number | undefined) => void;
  readonly today: IsoDate;
};

export function ConditionsStep({
  startDate,
  onStartDateChange,
  paymentDay,
  plannedEndDate,
  onPlannedEndDateChange,
  utilities,
  onUtilitiesChange,
  depositKopecks,
  onDepositChange,
  commissionKopecks,
  onCommissionChange,
  today,
}: ConditionsStepProps): JSX.Element {
  const [startPickerOpen, setStartPickerOpen] = useState(false);
  const [endPickerOpen, setEndPickerOpen] = useState(false);
  // «Сырые» набранные значения — источник отображения (группировка не
  // сбрасывает каретку); копейки едут в черновик для валидации.
  const [depositRaw, setDepositRaw] = useState(() =>
    depositKopecks === undefined ? '' : kopecksToAmountInputString(depositKopecks),
  );
  const [commissionRaw, setCommissionRaw] = useState(() =>
    commissionKopecks === undefined ? '' : kopecksToAmountInputString(commissionKopecks),
  );

  const startError = startDate === undefined ? undefined : rentalStartDateError(startDate, today);
  const endError = rentalPlannedEndDateError(plannedEndDate, startDate);
  const effectiveUtilities = utilities ?? 'included';

  return (
    <>
      <WizardHeading title="Условия аренды" />
      <div className="flex flex-col gap-8 px-6 pt-6">
        <PickerTriggerBox
          title="Начало аренды"
          required
          value={startDate === undefined ? undefined : formatDayMonthWithYear(startDate, today)}
          placeholder="Выбрать дату"
          icon={<Calendar className="h-6 w-6" />}
          onClick={() => setStartPickerOpen(true)}
          error={startError}
        />
        <PickerTriggerBox
          title="Окончание аренды"
          value={
            plannedEndDate === undefined
              ? undefined
              : formatDayMonthWithYear(plannedEndDate, today)
          }
          placeholder="Выбрать дату"
          icon={<Calendar className="h-6 w-6" />}
          onClick={() => setEndPickerOpen(true)}
          error={endError}
        />
        <UtilitiesPickerField
          value={effectiveUtilities}
          onChange={onUtilitiesChange}
        />
        <MoneyField
          title="Залог"
          raw={depositRaw}
          onRawChange={(raw) => {
            setDepositRaw(raw);
            onDepositChange(parseRublesToKopecks(raw));
          }}
          onClear={() => {
            setDepositRaw('');
            onDepositChange(undefined);
          }}
          ariaLabel="Залог, рублей"
        />
        <MoneyField
          title="Комиссия"
          raw={commissionRaw}
          onRawChange={(raw) => {
            setCommissionRaw(raw);
            onCommissionChange(parseRublesToKopecks(raw));
          }}
          onClear={() => {
            setCommissionRaw('');
            onCommissionChange(undefined);
          }}
          ariaLabel="Комиссия, рублей"
        />
      </div>

      {/* Рендер только в открытом состоянии — лента и черновик живут, пока
          пикер смонтирован (конвенция канона). */}
      {startPickerOpen && (
        <CalendarDatePicker
          title="Начало аренды"
          today={today}
          value={startDate ?? null}
          required
          onClose={() => setStartPickerOpen(false)}
          onConfirm={(date) => {
            onStartDateChange(date ?? undefined);
            setStartPickerOpen(false);
          }}
        />
      )}
      {endPickerOpen && (
        <CalendarDatePicker
          title="Окончание аренды"
          today={today}
          value={plannedEndDate ?? null}
          minDate={startDate !== undefined ? plannedEndDateMinDate(startDate, paymentDay) : undefined}
          onClose={() => setEndPickerOpen(false)}
          onConfirm={(date) => {
            onPlannedEndDateChange(date ?? undefined);
            setEndPickerOpen(false);
          }}
        />
      )}
    </>
  );
}
