'use client';

import type { JSX } from 'react';
import { Button } from '@/shared/ui/button';
import { propertyTypeOptions } from '@/features/properties/lib/property-types';
import type { PropertyType } from '@/entities/property/model/types';
import styles from './PropertyTypeStep.module.css';

export type PropertyTypeStepProps = {
  value?: PropertyType;
  onChange: (type: PropertyType) => void;
  onNext: () => void;
};

export function PropertyTypeStep({ value, onChange, onNext }: PropertyTypeStepProps): JSX.Element {
  return (
    <div className={styles.root}>
      <div className={styles.chips} role="group" aria-label="Тип объекта">
        {propertyTypeOptions.map((option) => (
          <Button
            key={option.value}
            type="button"
            variant={value === option.value ? 'primary' : 'secondary'}
            size="small"
            className={styles.chip}
            aria-pressed={value === option.value}
            onClick={() => onChange(option.value)}
          >
            {option.label}
          </Button>
        ))}
      </div>

      <Button
        type="button"
        variant="primary"
        size="large"
        fullWidth
        disabled={!value}
        onClick={onNext}
        className={styles.continue}
      >
        Продолжить
      </Button>
    </div>
  );
}
