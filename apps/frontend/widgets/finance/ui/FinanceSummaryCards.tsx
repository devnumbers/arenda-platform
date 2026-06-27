'use client';

import type { JSX } from 'react';
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
      <div className={styles.card}>
        <span className={styles.label}>Доходы</span>
        <span className={`${styles.value} ${styles.income}`}>
          {formatMoneyKopecks(incomeKopecks, { round: true })}
        </span>
      </div>
      <div className={styles.card}>
        <span className={styles.label}>Расходы</span>
        <span className={`${styles.value} ${styles.expense}`}>
          {formatMoneyKopecks(expenseKopecks, { round: true })}
        </span>
      </div>
      <div className={styles.card}>
        <span className={styles.label}>Прибыль</span>
        <span className={styles.value}>{formatMoneyKopecks(profitKopecks, { round: true })}</span>
      </div>
    </div>
  );
}
