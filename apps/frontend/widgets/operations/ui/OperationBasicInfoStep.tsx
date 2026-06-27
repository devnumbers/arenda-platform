'use client';

import { type ChangeEvent, type JSX } from 'react';
import { TextField } from '@/shared/ui/text-field';
import { LinkButton } from '@/shared/ui/link-button';
import { ROUTES } from '@/shared/config/routes';
import { type OperationType } from '@/entities/operation/model/types';
import { useProperties } from '@/features/properties/api';
import { FinanceErrorState } from '@/widgets/finance/ui/FinanceErrorState';
import { type BasicInfoData, type BasicInfoErrors } from '../model/types';
import { CategorySelect } from './CategorySelect';
import { PropertySelect } from './PropertySelect';
import styles from './OperationBasicInfoStep.module.css';

export type OperationBasicInfoStepProps = {
  readonly type: OperationType;
  readonly data: BasicInfoData;
  readonly onChange: (data: BasicInfoData) => void;
  readonly errors?: BasicInfoErrors;
  readonly readonly?: boolean;
};

export function OperationBasicInfoStep({
  type,
  data,
  onChange,
  errors,
  readonly,
}: OperationBasicInfoStepProps): JSX.Element {
  const {
    data: properties,
    isLoading: propertiesLoading,
    isError: propertiesError,
    isFetching: propertiesFetching,
    refetch: refetchProperties,
  } = useProperties();

  const isEmpty = !propertiesLoading && !propertiesError && properties?.length === 0;
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
      {propertiesError && (
        <FinanceErrorState
          onRetry={refetchProperties}
          isLoading={propertiesFetching}
        />
      )}
      <div className={styles.fields}>
        <TextField
          label="Сумма операции, ₽"
          placeholder="0"
          type="number"
          min={0}
          step="0.01"
          required
          fullWidth
          disabled={readonly}
          value={data.amount}
          onChange={handleAmountChange}
          error={errors?.amount}
        />
        <TextField
          label="Название операции"
          placeholder="Например, аренда за июнь"
          required
          maxLength={50}
          showCounter
          fullWidth
          disabled={readonly}
          value={data.name}
          onChange={handleNameChange}
          error={errors?.name}
        />
        <CategorySelect
          type={type}
          value={data.category}
          onChange={(category) => onChange({ ...data, category })}
          error={errors?.category}
          disabled={readonly}
        />
        <PropertySelect
          value={data.propertyId}
          onChange={(propertyId) => onChange({ ...data, propertyId })}
          error={errors?.property}
          disabled={readonly}
        />
        {isEmpty && (
          <LinkButton
            href={ROUTES.propertyNew}
            variant="secondary"
            size="medium"
            className={styles.emptyLink}
          >
            Сначала добавьте объект
          </LinkButton>
        )}
        <TextField
          label="Комментарий"
          placeholder="Дополнительная информация"
          multiline
          maxLength={500}
          showCounter
          fullWidth
          disabled={readonly}
          value={data.comment ?? ''}
          onChange={handleCommentChange}
          error={errors?.comment}
        />
      </div>
    </div>
  );
}
