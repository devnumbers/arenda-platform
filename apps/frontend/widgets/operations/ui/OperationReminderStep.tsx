'use client';

import { useId, type JSX } from 'react';
import clsx from 'clsx';
import { Checkbox } from '@heroui/react';
import { type ReminderData } from '../model/types';
import styles from './OperationReminderStep.module.css';

export type OperationReminderStepProps = {
  readonly data: ReminderData;
  readonly onChange: (data: ReminderData) => void;
  readonly readonly?: boolean;
};

const offsetOptions: { value: 1 | 3 | 7; label: string }[] = [
  { value: 1, label: 'За 1 день' },
  { value: 3, label: 'За 3 дня' },
  { value: 7, label: 'За 7 дней' },
];

export function OperationReminderStep({
  data,
  onChange,
  readonly,
}: OperationReminderStepProps): JSX.Element {
  const groupId = useId();

  return (
    <div className={styles.root}>
      <h2 className={styles.heading}>Напоминание</h2>
      <div className={styles.fields}>
        <Checkbox
          isSelected={data.enabled}
          onChange={(enabled) => onChange({ ...data, enabled })}
          isDisabled={readonly}
          className={styles.checkbox}
        >
          Добавить SMS-напоминание
        </Checkbox>

        {data.enabled && (
          <div
            className={styles.offsetGroup}
            role="radiogroup"
            aria-labelledby={`${groupId}-label`}
          >
            <span id={`${groupId}-label`} className={styles.offsetLabel}>
              За сколько дней напомнить
            </span>
            <div className={styles.offsetOptions}>
              {offsetOptions.map((option) => {
                const isSelected = data.offsetDays === option.value;
                return (
                  <button
                    key={option.value}
                    type="button"
                    role="radio"
                    aria-checked={isSelected}
                    disabled={readonly}
                    className={clsx(
                      styles.offsetOption,
                      isSelected && styles.selected
                    )}
                    onClick={() =>
                      onChange({ ...data, offsetDays: option.value })
                    }
                  >
                    <span className={styles.radio} aria-hidden="true">
                      <span className={styles.radioDot} />
                    </span>
                    {option.label}
                  </button>
                );
              })}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
