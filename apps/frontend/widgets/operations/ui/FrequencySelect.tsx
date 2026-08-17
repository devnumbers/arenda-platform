'use client';

import { useId, type JSX } from 'react';
import clsx from 'clsx';
import { type OperationFrequency } from '@/entities/operation';
import styles from './FrequencySelect.module.css';

export type FrequencySelectProps = {
  readonly value?: OperationFrequency;
  readonly onChange: (frequency: OperationFrequency) => void;
  readonly error?: string;
  readonly disabled?: boolean;
};

const frequencyOptions: { value: OperationFrequency; label: string }[] = [
  { value: 'once', label: 'Разово' },
  { value: 'monthly', label: 'Ежемесячно' },
  { value: 'yearly', label: 'Ежегодно' },
];

export function FrequencySelect({
  value,
  onChange,
  error,
  disabled,
}: FrequencySelectProps): JSX.Element {
  const labelId = useId();

  return (
    <div
      className={clsx(styles.root, error && styles.error)}
      role="radiogroup"
      aria-labelledby={labelId}
    >
      <span id={labelId} className={styles.label}>
        Периодичность
      </span>
      <div className={styles.options}>
        {frequencyOptions.map((option) => {
          const isSelected = value === option.value;
          return (
            <button
              key={option.value}
              type="button"
              role="radio"
              aria-checked={isSelected}
              disabled={disabled}
              className={clsx(styles.option, isSelected && styles.selected)}
              onClick={() => onChange(option.value)}
            >
              <span className={styles.radio} aria-hidden="true">
                <span className={styles.radioDot} />
              </span>
              {option.label}
            </button>
          );
        })}
      </div>
      {error && <span className={styles.errorText}>{error}</span>}
    </div>
  );
}
