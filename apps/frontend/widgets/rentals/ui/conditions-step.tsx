'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import { Calendar, SmallArrowDown } from '@/shared/assets/icons';
import type { IsoDate } from '@/shared/lib/calendar';
import { formatDayMonthWithYear } from '@/shared/lib/date-format';
import { kopecksToAmountInputString, parseRublesToKopecks } from '@/shared/lib/format-money';
import type { RentalUtilities } from '@/entities/rental';
import {
  UTILITIES_OPTIONS,
  rentalPlannedEndDateError,
  rentalStartDateError,
  utilitiesLabel,
} from '@/features/rentals';
import { CalendarDatePicker, PickerMenu, type PickerMenuGroup } from '@/shared/ui/design';
import { FieldTitle, MoneyField, PickerTriggerBox, WizardHeading } from './wizard-chrome';

/**
 * Шаг 3 «Условия аренды» (Figma 1270:46821/47960, шит даты 1270:37644):
 * начало* и опциональное окончание открывают канонический бесконечный
 * календарь CalendarDatePicker, коммунальные платежи — PickerMenu
 * (радио-меню макета 1296:48965: «Включены в стоимость» / «Только
 * счетчики» / «Вся квитанция»), залог и комиссия — необязательные денежные
 * поля. Дата в поле — канонический формат «10 мая, 2027».
 */

export type ConditionsStepProps = {
  readonly startDate: IsoDate | undefined;
  readonly onStartDateChange: (startDate: IsoDate | undefined) => void;
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

/** Поле режима коммунальных платежей: триггер-бокс + PickerMenu (меню на
 * десктопе, шит на мобиле — канон выбора одной опции, выбор применяется
 * сразу). */
function UtilitiesPickerField({
  value,
  onChange,
}: {
  readonly value: RentalUtilities;
  readonly onChange: (utilities: RentalUtilities) => void;
}): JSX.Element {
  const groups: ReadonlyArray<PickerMenuGroup> = [
    {
      options: UTILITIES_OPTIONS.map((option) => ({
        label: option.label,
        selected: value === option.value,
        onSelect: () => onChange(option.value),
      })),
    },
  ];

  return (
    <div className="flex w-full flex-col gap-2 font-sans">
      <FieldTitle title="Коммунальные платежи" />
      <PickerMenu title="Коммунальные платежи" groups={groups}>
        <button
          type="button"
          aria-label={`Коммунальные платежи: ${utilitiesLabel(value)}`}
          className="group/trigger flex h-14 w-full cursor-pointer items-center rounded-button bg-surface-muted py-0 pl-[18px] pr-2 text-left transition-shadow outline-none hover:shadow-[inset_0_0_0_2px_var(--dl-input-border)]"
        >
          <span className="min-w-0 flex-1 truncate text-base leading-[18px] text-content">
            {utilitiesLabel(value)}
          </span>
          {/* Хвостовая иконка — IconButton-primary-анатомия (круг 44 с
              hover-подложкой), как у триггеров дат. */}
          <span
            aria-hidden
            className="flex h-11 w-11 shrink-0 items-center justify-center rounded-pill text-content transition-colors group-hover/trigger:bg-surface-muted group-active/trigger:bg-surface-muted-hover"
          >
            <SmallArrowDown className="h-6 w-6" />
          </span>
        </button>
      </PickerMenu>
    </div>
  );
}
