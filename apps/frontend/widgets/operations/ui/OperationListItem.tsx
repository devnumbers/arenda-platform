'use client';

import type { JSX } from 'react';
import NextLink from 'next/link';
import type { components } from '@/shared/api/generated';
import { Home } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { getOperationDueInfo } from '@/entities/operation/lib/dates';
import { formatMoneyKopecks } from '@/entities/operation/lib/formatMoney';
import styles from './OperationListItem.module.css';

type OperationResponse = components['schemas']['OperationResponse'];

export type OperationListItemProps = {
  readonly operation: OperationResponse;
};

export function OperationListItem({ operation }: OperationListItemProps): JSX.Element {
  const isIncome = operation.type === 'income';
  const sign = isIncome ? '+' : '-';
  const amountClass = isIncome ? styles.amountIncome : styles.amountExpense;
  const { subtitle, trailing } = getOperationDueInfo(operation);

  return (
    <NextLink href={ROUTES.financeOperation(operation.id)} className={styles.root}>
      <span className={styles.iconCircle}>
        <Home />
      </span>
      <span className={styles.main}>
        <span className={styles.name}>{operation.name}</span>
        {subtitle && <span className={styles.subtitle}>{subtitle}</span>}
      </span>
      <span className={styles.right}>
        <span className={`${styles.amount} ${amountClass}`}>
          {sign}
          {formatMoneyKopecks(operation.amount_kopecks, { round: true })}
        </span>
        <span className={styles.trailing}>{trailing}</span>
      </span>
    </NextLink>
  );
}
