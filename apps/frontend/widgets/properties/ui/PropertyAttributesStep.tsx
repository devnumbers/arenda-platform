'use client';

import { type JSX, useCallback, useMemo, useState } from 'react';
import { Button } from '@/shared/ui/button';
import type { PropertyType, PropertyAttributes } from '@/entities/property/model/types';
import {
  PropertyAttributesFields,
  validateAttributes,
  type AttrErrors,
} from '@/features/property-attributes';
import styles from './PropertyAttributesStep.module.css';

export type PropertyAttributesStepProps = {
  readonly type: PropertyType;
  readonly value: PropertyAttributes;
  readonly onChange: (next: PropertyAttributes) => void;
  readonly onNext: () => void;
  readonly onBack?: () => void;
};

export function PropertyAttributesStep({
  type,
  value,
  onChange,
  onNext,
}: PropertyAttributesStepProps): JSX.Element {
  const [errors, setErrors] = useState<AttrErrors>({});

  const liveErrors = useMemo(() => validateAttributes(type, value), [type, value]);
  const isValid = Object.keys(liveErrors).length === 0;
  const isEmpty = Object.keys(value).length === 0;

  const handleFieldBlur = useCallback(() => {
    setErrors(validateAttributes(type, value));
  }, [type, value]);

  const handleNext = useCallback(() => {
    if (isEmpty) {
      onNext();
      return;
    }
    setErrors(liveErrors);
    if (isValid) {
      onNext();
    }
  }, [isEmpty, liveErrors, isValid, onNext]);

  return (
    <div className={styles.root}>
      <h2 className={styles.heading}>Характеристики</h2>
      <p className={styles.subtitle}>Все поля необязательные</p>
      <PropertyAttributesFields
        type={type}
        value={value}
        onChange={onChange}
        errors={errors}
        onFieldBlur={handleFieldBlur}
      />
      <Button
        type="button"
        variant="primary"
        size="large"
        fullWidth
        className={styles.continue}
        disabled={!isEmpty && !isValid}
        onClick={handleNext}
      >
        {isEmpty ? 'Пропустить' : 'Далее'}
      </Button>
    </div>
  );
}
