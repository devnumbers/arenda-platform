'use client';

import type { JSX } from 'react';
import { Card } from '@heroui/react/card';
import { Skeleton } from '@heroui/react/skeleton';
import { LinkButton } from '@/shared/ui/link-button';
import { BoldWallet } from '@/shared/assets/icons';
import type { components } from '@/shared/api/generated';
import { useOperationsForProperties } from '../lib/use-operations-for-properties';
import { aggregateOperations } from '../lib/finance-aggregator';
import { formatMoney } from '../lib/format-money';
import { EmptyState } from './EmptyState';
import styles from './FinanceSection.module.css';

type PropertyResponse = components['schemas']['PropertyResponse'];

type FinanceSectionProps = {
  readonly properties: PropertyResponse[] | undefined;
  readonly isLoading: boolean;
};

export function FinanceSection({
  properties,
  isLoading,
}: FinanceSectionProps): JSX.Element {
  const { operationsList, isLoading: operationsLoading } =
    useOperationsForProperties(properties);
  const showLoading = isLoading || operationsLoading;

  const { incomeKopecks, expenseKopecks, profitKopecks } =
    aggregateOperations(operationsList);

  if (showLoading) {
    return (
      <section className={styles.section}>
        <Skeleton className={styles.titleSkeleton} />
        <Card className={styles.card}>
          <Skeleton className={styles.profitSkeleton} />
          <div className={styles.columnsSkeleton}>
            <Skeleton className={styles.columnSkeleton} />
            <Skeleton className={styles.columnSkeleton} />
          </div>
        </Card>
      </section>
    );
  }

  if (operationsList.length === 0) {
    return (
      <section className={styles.section}>
        <h2 className={styles.title}>Финансы и операции</h2>
        <EmptyState
          icon={<BoldWallet />}
          entities="операций"
          subtitle="Добавьте доход или расход"
          actionHref="/finance"
          actionText="Добавить операцию"
        />
      </section>
    );
  }

  return (
    <section className={styles.section}>
      <h2 className={styles.title}>Финансы и операции</h2>
      <Card className={styles.card}>
        <div className={styles.profit}>
          <span className={styles.profitLabel}>Прибыль</span>
          <span className={styles.profitValue}>
            {formatMoney(profitKopecks)}
          </span>
        </div>
        <div className={styles.columns}>
          <div className={styles.column}>
            <span className={styles.columnLabel}>Доход</span>
            <span className={styles.columnValueIncome}>
              {formatMoney(incomeKopecks)}
            </span>
          </div>
          <div className={styles.column}>
            <span className={styles.columnLabel}>Расход</span>
            <span className={styles.columnValueExpense}>
              {formatMoney(expenseKopecks)}
            </span>
          </div>
        </div>
        <LinkButton
          href="/finance"
          variant="secondary"
          size="medium"
          fullWidth
        >
          Все операции
        </LinkButton>
      </Card>
    </section>
  );
}
