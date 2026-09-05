'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import { ArrowRight } from '@/shared/assets/icons';
import type { IsoDate } from '@/entities/payment';
import { formatDayMonthWithYear } from '@/entities/payment';
import { CalendarDatePicker, ListRow } from '@/shared/ui/design';
import { WizardHeading } from './wizard-chrome';

/**
 * Шаг 4 визарда — «Окончание платежа» (Figma 843:8345/843-8358): строка
 * «Выбрать дату» (с датой — она сама) открывает канонический бесконечный
 * календарь CalendarDatePicker (как выбор даты в задачах; решение
 * владельца 2026-09-04 вместо модалки с календарём одного месяца):
 * лента месяцев вперёд, прошлого нет, «Выбрать» коммитит дату, снятая
 * (null) — бессрочно. Напоминания и email-уведомления макета — вне среза
 * (в контракте платежа их нет).
 */

export type EndDateStepProps = {
  readonly endDate: IsoDate | undefined;
  readonly onEndDateChange: (endDate: IsoDate | undefined) => void;
  readonly today: IsoDate;
  /** Заголовок шага рисует хост (шит правки #467 несёт его в ModalContent). */
  readonly withHeading?: boolean;
};

export function EndDateStep({
  endDate,
  onEndDateChange,
  today,
  withHeading = true,
}: EndDateStepProps): JSX.Element {
  const [pickerOpen, setPickerOpen] = useState(false);

  return (
    <>
      {withHeading && (
        <WizardHeading
          title="Окончание платежа"
          subtitle="После выбранной даты, платеж перестанет оплачиваться и удалится. Выбирать необязательно"
        />
      )}
      <div className="flex flex-col pt-2">
        <ListRow
          title={endDate !== undefined ? formatDayMonthWithYear(endDate, today) : 'Выбрать дату'}
          className="py-4"
          onSelect={() => setPickerOpen(true)}
          trailing={<ArrowRight className="h-6 w-6" aria-hidden />}
        />
      </div>

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
