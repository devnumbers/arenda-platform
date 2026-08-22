'use client';

import type { JSX } from 'react';
import clsx from 'clsx';
import NextLink from 'next/link';
import { ROUTES } from '@/shared/config/routes';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import styles from './FinanceSummaryCards.module.css';

export type FinanceSummaryCardsProps = {
  readonly incomeKopecks: number;
  readonly expenseKopecks: number;
  readonly profitKopecks: number;
};

export function FinanceSummaryCards({
  incomeKopecks,
  expenseKopecks,
  profitKopecks,
}: FinanceSummaryCardsProps): JSX.Element {
  return (
    <div className={styles.root}>
      <NextLink
        href={`${ROUTES.financeOperations}?type=income&period=all`}
        className={styles.card}
        aria-label="Перейти к операциям: Доходы"
      >
        <span className={styles.label}>Доходы</span>
        <span className={clsx(styles.value, styles.income)}>
          {formatMoneyKopecks(incomeKopecks, { round: true })}
        </span>
      </NextLink>
      <NextLink
        href={`${ROUTES.financeOperations}?type=expense&period=all`}
        className={styles.card}
        aria-label="Перейти к операциям: Расходы"
      >
        <span className={styles.label}>Расходы</span>
        <span className={clsx(styles.value, styles.expense)}>
          {formatMoneyKopecks(expenseKopecks, { round: true })}
        </span>
      </NextLink>
      <div className={styles.card}>
        <span className={styles.label}>Прибыль</span>
        <span className={styles.value}>{formatMoneyKopecks(profitKopecks, { round: true })}</span>
      </div>
    </div>
  );
}
