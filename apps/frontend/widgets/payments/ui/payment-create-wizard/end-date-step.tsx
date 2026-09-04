'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import { ArrowRight } from '@/shared/assets/icons';
import type { IsoDate } from '@/entities/payment';
import { formatDayMonthWithYear } from '@/entities/payment';
import { ListRow, Modal, ModalContent } from '@/shared/ui/design';
import { WizardHeading } from './wizard-chrome';
import { YearMonthCalendar } from './periodicity-step';

/**
 * Шаг 4 визарда — «Окончание платежа» (Figma 843:8345/843-8358): строка
 * «Выбрать дату» (с датой — она сама) открывает модалку с годовым
 * календарём — чип «Месяц Год» с колесами (годы не раньше текущего) и
 * календарем одного месяца; выбор дня закрывает модалку и возвращает дату.
 * Пусто — бессрочный. Напоминания и email-уведомления макета — вне среза
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

      <Modal open={pickerOpen} onOpenChange={setPickerOpen}>
        <ModalContent title="Выбрать дату" titleSrOnly>
          <YearMonthCalendar
            padded={false}
            value={
              endDate === undefined
                ? undefined
                : {
                    year: Number(endDate.slice(0, 4)),
                    month0: Number(endDate.slice(5, 7)) - 1,
                    day: Number(endDate.slice(8)),
                  }
            }
            today={today}
            minDate={today}
            onPick={(picked) => {
              const mm = String(picked.month0 + 1).padStart(2, '0');
              const dd = String(picked.day).padStart(2, '0');
              onEndDateChange(`${picked.year}-${mm}-${dd}`);
              setPickerOpen(false);
            }}
          />
        </ModalContent>
      </Modal>
    </>
  );
}
