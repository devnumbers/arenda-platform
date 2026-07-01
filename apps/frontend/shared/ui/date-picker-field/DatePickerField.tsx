'use client';

import type { JSX } from 'react';
import { parseDate, type CalendarDate } from '@internationalized/date';
import { DatePicker } from '@heroui/react/date-picker';
import { DateField } from '@heroui/react/date-field';
import { Calendar } from '@heroui/react/calendar';
import { Label } from '@heroui/react/label';
import clsx from 'clsx';
import styles from './DatePickerField.module.css';

export type DatePickerFieldProps = {
  readonly label: string;
  readonly value?: string;
  readonly onChange: (value: string) => void;
  readonly required?: boolean;
  readonly disabled?: boolean;
  readonly error?: string;
  readonly minValue?: string;
  readonly maxValue?: string;
  readonly fullWidth?: boolean;
  readonly className?: string;
};

function parseOptionalDate(value: string | undefined): CalendarDate | null {
  if (!value) return null;
  try {
    return parseDate(value);
  } catch {
    return null;
  }
}

export function DatePickerField({
  label,
  value,
  onChange,
  required,
  disabled,
  error,
  minValue,
  maxValue,
  fullWidth,
  className,
}: DatePickerFieldProps): JSX.Element {
  const handleChange = (date: CalendarDate | null) => {
    onChange(date?.toString() ?? '');
  };

  return (
    <div className={clsx(styles.root, fullWidth && styles.fullWidth, className)}>
      <DatePicker
        value={parseOptionalDate(value)}
        onChange={handleChange}
        minValue={parseOptionalDate(minValue) ?? undefined}
        maxValue={parseOptionalDate(maxValue) ?? undefined}
        isDisabled={disabled}
        aria-label={label}
        className={styles.picker}
      >
        <Label className={styles.label}>
          {label}
          {required && <span className={styles.required}>*</span>}
        </Label>
        <DateField.Group>
          <DateField.Input>
            {(segment) => <DateField.Segment segment={segment} />}
          </DateField.Input>
          <DateField.Suffix>
            <DatePicker.Trigger>
              <DatePicker.TriggerIndicator />
            </DatePicker.Trigger>
          </DateField.Suffix>
        </DateField.Group>
        <DatePicker.Popover>
          <Calendar aria-label={label}>
            <Calendar.Header>
              <Calendar.YearPickerTrigger>
                <Calendar.YearPickerTriggerHeading />
                <Calendar.YearPickerTriggerIndicator />
              </Calendar.YearPickerTrigger>
              <Calendar.NavButton slot="previous" />
              <Calendar.NavButton slot="next" />
            </Calendar.Header>
            <Calendar.Grid>
              <Calendar.GridHeader>
                {(day) => <Calendar.HeaderCell>{day}</Calendar.HeaderCell>}
              </Calendar.GridHeader>
              <Calendar.GridBody>
                {(date) => <Calendar.Cell date={date} />}
              </Calendar.GridBody>
            </Calendar.Grid>
          </Calendar>
        </DatePicker.Popover>
      </DatePicker>
      {error && <span className={styles.error}>{error}</span>}
    </div>
  );
}
