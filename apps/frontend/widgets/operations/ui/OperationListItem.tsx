'use client';

import type { JSX } from 'react';
import NextLink from 'next/link';
import type { components } from '@/shared/api/generated';
import { ROUTES } from '@/shared/config/routes';
import { getOperationStatusLabel, operationStatusOptions } from '@/entities/operation/lib/statuses';
import { formatOperationDate } from '@/entities/operation/lib/dates';
import { formatMoneyKopecks } from '@/entities/operation/lib/formatMoney';
import styles from './OperationListItem.module.css';

type OperationResponse = components['schemas']['OperationResponse'];

export type OperationListItemVariant = 'default' | 'dashboard';

export type OperationListItemProps = {
  readonly operation: OperationResponse;
  readonly propertyName?: string;
  readonly showProperty?: boolean;
  readonly variant?: OperationListItemVariant;
};

const variantClassMap: Record<string, string> = {
  warning: styles.badgeWarning,
  danger: styles.badgeDanger,
  success: styles.badgeSuccess,
  default: styles.badgeDefault,
};

export function OperationListItem({
  operation,
  propertyName,
  showProperty = false,
  variant = 'default',
}: OperationListItemProps): JSX.Element {
  const isDashboard = variant === 'dashboard';
  const isIncome = operation.type === 'income';
  const sign = isIncome ? '+' : '-';
  const amountClass = isIncome ? styles.amountIncome : styles.amountExpense;
  const statusLabel = getOperationStatusLabel(operation.status);
  const statusVariant = operationStatusOptions.find((option) => option.value === operation.status)?.variant ?? 'default';
  const metaItems = [formatOperationDate(operation.operation_date), operation.category_name];

  if (showProperty && propertyName) {
    metaItems.push(propertyName);
  }

  return (
    <NextLink
      href={ROUTES.financeOperation(operation.id)}
      className={`${styles.root} ${isDashboard ? styles.rootDashboard : ''}`}
    >
      <div className={styles.main}>
        <span className={styles.nameRow}>
          <span className={styles.name}>{operation.name}</span>
        </span>
        <span className={styles.meta}>{metaItems.join(' · ')}</span>
      </div>
      <div className={styles.right}>
        <span className={`${styles.amount} ${amountClass}`}>
          {sign}
          {formatMoneyKopecks(operation.amount_kopecks, { round: true })}
        </span>
        {isDashboard ? (
          <span className={`${styles.badge} ${variantClassMap[statusVariant]}`}>{statusLabel}</span>
        ) : (
          <span className={styles.status}>{statusLabel}</span>
        )}
      </div>
    </NextLink>
  );
}
