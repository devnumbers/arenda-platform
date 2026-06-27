'use client';

import { type ChangeEvent, type JSX } from 'react';
import { TextField } from '@/shared/ui/text-field';
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

  const handleDateChange = (event: ChangeEvent<HTMLInputElement>) => {
    onChange({ ...data, date: event.currentTarget.value || undefined });
  };

  const handlePaymentDayChange = (event: ChangeEvent<HTMLInputElement>) => {
    const value = event.currentTarget.value;
    onChange({
      ...data,
      paymentDay: value === '' ? undefined : Number(value),
    });
  };

  const handleEndDateChange = (event: ChangeEvent<HTMLInputElement>) => {
    onChange({ ...data, endDate: event.currentTarget.value || undefined });
  };

  return (
    <div className={styles.root}>
      <h2 className={styles.heading}>Дата и периодичность</h2>
      <div className={styles.fields}>
        <FrequencySelect
          value={data.frequency}
          onChange={(frequency) => onChange({ ...data, frequency })}
          error={errors?.frequency}
        />
        <TextField
          label={recurring ? 'Дата первого повтора' : 'Дата операции'}
          type="date"
          required
          fullWidth
          disabled={readonly}
          value={data.date ?? ''}
          onChange={handleDateChange}
          error={errors?.date}
        />
        {recurring && (
          <>
            <TextField
              label="День операции"
              placeholder="1–31"
              type="number"
              min={1}
              max={31}
              required
              fullWidth
              disabled={readonly}
              value={data.paymentDay ?? ''}
              onChange={handlePaymentDayChange}
              error={errors?.paymentDay}
            />
            <TextField
              label="Дата окончания (необязательно)"
              type="date"
              fullWidth
              disabled={readonly}
              value={data.endDate ?? ''}
              onChange={handleEndDateChange}
              error={errors?.endDate}
            />
          </>
        )}
      </div>
    </div>
  );
}
