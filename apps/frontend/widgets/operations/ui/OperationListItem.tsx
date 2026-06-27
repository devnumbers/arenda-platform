'use client';

import type { JSX } from 'react';
import NextLink from 'next/link';
import type { components } from '@/shared/api/generated';
import { ROUTES } from '@/shared/config/routes';
import { getOperationStatusLabel } from '@/entities/operation/lib/statuses';
import { getCategoryLabel } from '@/entities/operation/lib/categories';
import { formatOperationDate } from '@/entities/operation/lib/dates';
import { formatMoneyKopecks } from '@/entities/operation/lib/formatMoney';
import styles from './OperationListItem.module.css';

type OperationResponse = components['schemas']['OperationResponse'];

type OperationWithPropertyName = OperationResponse & {
  property_name?: string;
};

export type OperationListItemProps = {
  readonly operation: OperationResponse;
  readonly showProperty?: boolean;
};

export function OperationListItem({
  operation,
  showProperty = false,
}: OperationListItemProps): JSX.Element {
  const operationWithProperty = operation as OperationWithPropertyName;
  const isIncome = operation.type === 'income';
  const sign = isIncome ? '+' : '-';
  const amountClass = isIncome ? styles.amountIncome : styles.amountExpense;
  const statusLabel = getOperationStatusLabel(operation.status);
  const metaItems = [formatOperationDate(operation.operation_date), getCategoryLabel(operation.category)];

  if (showProperty && operationWithProperty.property_name) {
    metaItems.push(operationWithProperty.property_name);
  }

  return (
    <NextLink
      href={ROUTES.financeOperation(operation.id)}
      className={styles.root}
    >
      <div className={styles.main}>
        <span className={styles.name}>{operation.name}</span>
        <span className={styles.meta}>{metaItems.join(' · ')}</span>
      </div>
      <div className={styles.right}>
        <span className={`${styles.amount} ${amountClass}`}>
          {sign}
          {formatMoneyKopecks(operation.amount_kopecks, { round: true })}
        </span>
        <span className={styles.status}>{statusLabel}</span>
      </div>
    </NextLink>
  );
}
