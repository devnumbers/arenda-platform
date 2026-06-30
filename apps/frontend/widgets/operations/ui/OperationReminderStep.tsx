'use client';

import { type JSX } from 'react';
import { type ReminderData } from '../model/types';
import { ReminderSection } from './ReminderSection';
import styles from './OperationReminderStep.module.css';

export type OperationReminderStepProps = {
  readonly data: ReminderData;
  readonly onChange: (data: ReminderData) => void;
  readonly readonly?: boolean;
};

export function OperationReminderStep({
  data,
  onChange,
  readonly,
}: OperationReminderStepProps): JSX.Element {
  return (
    <div className={styles.root}>
      <h2 className={styles.heading}>Напоминание</h2>
      <div className={styles.fields}>
        <ReminderSection
          enabled={data.enabled}
          offsetDays={data.offsetDays}
          onEnabledChange={(enabled) => onChange({ ...data, enabled })}
          onOffsetChange={(offsetDays) => onChange({ ...data, offsetDays })}
          disabled={readonly}
        />
      </div>
    </div>
  );
}
