'use client';

import type { JSX } from 'react';
import { useId, useState } from 'react';
import { parseDate, type CalendarDate } from '@internationalized/date';
import {
  Popover,
  PopoverContent,
  PopoverDialog,
  PopoverTrigger,
} from '@heroui/react/popover';
import { Calendar } from '@heroui/react/calendar';
import { Button } from '@/shared/ui/button';
import styles from './PaymentDayPicker.module.css';

const REFERENCE_DATE = '2026-01-01';
const MIN_DATE = '2026-01-01';
const MAX_DATE = '2026-01-31';

function dayToDate(day: number | undefined): CalendarDate | null {
  if (day === undefined || day < 1 || day > 31) return null;
  return parseDate(`${REFERENCE_DATE.slice(0, 8)}${String(day).padStart(2, '0')}`);
}

export type PaymentDayPickerProps = {
  readonly value?: number;
  readonly onChange: (day: number) => void;
};

export function PaymentDayPicker({ value, onChange }: PaymentDayPickerProps): JSX.Element {
  const [isOpen, setIsOpen] = useState(false);
  const triggerId = useId();

  const handleChange = (date: CalendarDate | null) => {
    if (!date) return;
    onChange(date.day);
    setIsOpen(false);
  };

  return (
    <div className={styles.root}>
      <label htmlFor={triggerId} className={styles.label}>
        День оплаты <span className={styles.required}>*</span>
      </label>
      <Popover isOpen={isOpen} onOpenChange={setIsOpen}>
        <PopoverTrigger>
          <Button
            id={triggerId}
            type="button"
            variant="secondary"
            size="large"
            fullWidth
            aria-haspopup="dialog"
            aria-expanded={isOpen}
            className={styles.trigger}
          >
            {value ? `${value}-е число` : 'Выбрать'}
          </Button>
        </PopoverTrigger>
        <PopoverContent className={styles.popover}>
          <PopoverDialog aria-label="День оплаты">
            <Calendar
              aria-label="Выбрать день оплаты"
              value={dayToDate(value)}
              onChange={handleChange}
              defaultFocusedValue={parseDate(REFERENCE_DATE)}
              minValue={parseDate(MIN_DATE)}
              maxValue={parseDate(MAX_DATE)}
            >
              <Calendar.Grid>
                <Calendar.GridHeader>
                  {(day) => <Calendar.HeaderCell>{day}</Calendar.HeaderCell>}
                </Calendar.GridHeader>
                <Calendar.GridBody>
                  {(date) => <Calendar.Cell date={date} />}
                </Calendar.GridBody>
              </Calendar.Grid>
            </Calendar>
          </PopoverDialog>
        </PopoverContent>
      </Popover>
    </div>
  );
}
