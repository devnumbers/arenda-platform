'use client';

import type { JSX } from 'react';
import { Button } from '@/shared/ui/button';
import { type OperationType } from '@/entities/operation/model/types';
import styles from './TypeSelect.module.css';

export type TypeSelectProps = {
  readonly value: OperationType;
  readonly onChange: (type: OperationType) => void;
};

const OPTIONS: { readonly value: OperationType; readonly label: string }[] = [
  { value: 'income', label: 'Доход' },
  { value: 'expense', label: 'Расход' },
];

export function TypeSelect({ value, onChange }: TypeSelectProps): JSX.Element {
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
            onClick={() => onChange(option.value)}
          >
            {option.label}
          </Button>
        ))}
      </div>
    </div>
  );
}
