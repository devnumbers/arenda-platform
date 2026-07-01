'use client';

import type { JSX } from 'react';
import { DatePickerField } from '@/shared/ui/date-picker-field';
import { type ScheduleData, type ScheduleErrors } from '../model/types';
import { FrequencySelect } from './FrequencySelect';
import styles from './OperationScheduleStep.module.css';

export type OperationScheduleStepProps = {
  readonly data: ScheduleData;
  readonly onChange: (data: ScheduleData) => void;
  readonly errors?: ScheduleErrors;
  readonly readonly?: boolean;
};

function isRecurring(frequency: string): boolean {
  return frequency !== 'once';
}

export function OperationScheduleStep({
  data,
  onChange,
  errors,
  readonly,
}: OperationScheduleStepProps): JSX.Element {
  const recurring = isRecurring(data.frequency);

  const handleDateChange = (value: string) => {
    onChange({ ...data, date: value || undefined });
  };

  const handleEndDateChange = (value: string) => {
    onChange({ ...data, endDate: value || undefined });
  };

  return (
    <div className={styles.root}>
      <h2 className={styles.heading}>Дата и периодичность</h2>
      <div className={styles.fields}>
        <FrequencySelect
          value={data.frequency}
          onChange={(frequency) => onChange({ ...data, frequency })}
          error={errors?.frequency}
          disabled={readonly}
        />
        <DatePickerField
          label={recurring ? 'Дата первого повтора' : 'Дата операции'}
          value={data.date ?? ''}
          onChange={handleDateChange}
          required
          disabled={readonly}
          fullWidth
          error={errors?.date}
        />
        {recurring && (
          <DatePickerField
            label="Дата окончания (необязательно)"
            value={data.endDate ?? ''}
            onChange={handleEndDateChange}
            disabled={readonly}
            fullWidth
            error={errors?.endDate}
          />
        )}
      </div>
    </div>
  );
}
