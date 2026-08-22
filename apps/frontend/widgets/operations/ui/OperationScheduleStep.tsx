'use client';

import type { JSX } from 'react';
import { DateSelect } from '@/shared/ui/date-select';
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
        <DateSelect
          label={recurring ? 'Дата первого повтора' : 'Дата операции'}
          value={data.date}
          placeholder="Выбрать дату"
          required
          disabled={readonly}
          error={errors?.date}
          onChange={(v) => onChange({ ...data, date: v === '' ? undefined : v })}
        />
        {recurring && (
          <DateSelect
            label="Дата окончания (необязательно)"
            value={data.endDate}
            placeholder="Выбрать дату"
            minValue={data.date}
            disabled={readonly}
            error={errors?.endDate}
            onChange={(v) => onChange({ ...data, endDate: v === '' ? undefined : v })}
          />
        )}
      </div>
    </div>
  );
}
