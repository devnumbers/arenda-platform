'use client';

import { type ChangeEvent, type JSX } from 'react';
import { TextField } from '@/shared/ui/text-field';
import { type OperationType } from '@/entities/operation/model/types';
import { type BasicInfoData, type BasicInfoErrors } from '../model/types';
import { CategorySelect } from './CategorySelect';
import { PropertySelect } from './PropertySelect';
import styles from './OperationBasicInfoStep.module.css';

export type OperationBasicInfoStepProps = {
  readonly type: OperationType;
  readonly data: BasicInfoData;
  readonly onChange: (data: BasicInfoData) => void;
  readonly errors?: BasicInfoErrors;
};

export function OperationBasicInfoStep({
  type,
  data,
  onChange,
  errors,
}: OperationBasicInfoStepProps): JSX.Element {
  const handleAmountChange = (event: ChangeEvent<HTMLInputElement>) => {
    onChange({ ...data, amount: event.currentTarget.value });
  };

  const handleNameChange = (event: ChangeEvent<HTMLInputElement>) => {
    onChange({ ...data, name: event.currentTarget.value });
  };

  const handleCommentChange = (event: ChangeEvent<HTMLTextAreaElement>) => {
    onChange({ ...data, comment: event.currentTarget.value });
  };

  return (
    <div className={styles.root}>
      <h2 className={styles.heading}>Основная информация</h2>
      <div className={styles.fields}>
        <TextField
          label="Сумма платежа, ₽"
          placeholder="0"
          type="number"
          min={0}
          step="0.01"
          required
          fullWidth
          value={data.amount}
          onChange={handleAmountChange}
          error={errors?.amount}
        />
        <TextField
          label="Название платежа"
          placeholder="Например, аренда за июнь"
          required
          maxLength={50}
          showCounter
          fullWidth
          value={data.name}
          onChange={handleNameChange}
          error={errors?.name}
        />
        <CategorySelect
          type={type}
          value={data.category}
          onChange={(category) => onChange({ ...data, category })}
          error={errors?.category}
        />
        <PropertySelect
          value={data.propertyId}
          onChange={(propertyId) => onChange({ ...data, propertyId })}
          error={errors?.property}
        />
        <TextField
          label="Комментарий"
          placeholder="Дополнительная информация"
          multiline
          maxLength={500}
          showCounter
          fullWidth
          value={data.comment ?? ''}
          onChange={handleCommentChange}
          error={errors?.comment}
        />
      </div>
    </div>
  );
}
