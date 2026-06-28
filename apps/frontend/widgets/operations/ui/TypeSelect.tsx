'use client';

import type { JSX } from 'react';
import { Button } from '@/shared/ui/button';
import { type OperationType } from '@/entities/operation/model/types';
import styles from './TypeSelect.module.css';

export type TypeSelectProps = {
  readonly value: OperationType;
  readonly onChange: (type: OperationType) => void;
  readonly disabled?: boolean;
};

const OPTIONS: { readonly value: OperationType; readonly label: string }[] = [
  { value: 'income', label: 'Доход' },
  { value: 'expense', label: 'Расход' },
];

export function TypeSelect({ value, onChange, disabled }: TypeSelectProps): JSX.Element {
  return (
    <div className={styles.root}>
      <span className={styles.label}>Тип операции</span>
      <div className={styles.buttons}>
        {OPTIONS.map((option) => (
          <Button
            key={option.value}
            type="button"
            variant={value === option.value ? 'secondary' : 'icon-black'}
            size="medium"
            fullWidth
            disabled={disabled}
            onClick={() => onChange(option.value)}
          >
            {option.label}
          </Button>
        ))}
      </div>
    </div>
  );
}
