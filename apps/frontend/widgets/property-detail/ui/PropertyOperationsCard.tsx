'use client';

import type { JSX } from 'react';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import type { components } from '@/shared/api/generated';
import { PropertyDetailSection } from './PropertyDetailSection';
import styles from './PropertyOperationsCard.module.css';

type PropertyOperationsSummaryResponse =
  components['schemas']['PropertyOperationsSummaryResponse'];

export type PropertyOperationsCardProps = {
  readonly propertyName: string;
  readonly summary: PropertyOperationsSummaryResponse | undefined;
};

export function PropertyOperationsCard({
  propertyName,
  summary,
}: PropertyOperationsCardProps): JSX.Element {
  const monthlyProfit = summary?.monthly_profit_kopecks ?? 0;
  const allTimeProfit = summary?.all_time_profit_kopecks ?? 0;
  const isEmpty = !summary || (monthlyProfit === 0 && allTimeProfit === 0);

  return (
    <PropertyDetailSection>
      <div className={styles.header}>
        <h2 className={styles.title}>Операции объекта</h2>
        <span className={styles.propertyName}>{propertyName}</span>
      </div>

      {isEmpty ? (
        <p className={styles.emptyText}>
          Операций ещё не было. Здесь будет отображаться прибыль по объекту.
        </p>
      ) : (
        <div className={styles.card}>
          <div className={styles.row}>
            <span className={styles.label}>Прибыль за месяц</span>
            <span className={styles.value}>
              {formatMoneyKopecks(monthlyProfit)}
            </span>
          </div>
          <div className={styles.row}>
            <span className={styles.label}>Прибыль за всё время</span>
            <span className={styles.value}>
              {formatMoneyKopecks(allTimeProfit)}
            </span>
          </div>
        </div>
      )}
    </PropertyDetailSection>
  );
}
