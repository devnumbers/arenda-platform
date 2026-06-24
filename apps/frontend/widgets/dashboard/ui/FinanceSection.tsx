'use client';

import type { JSX } from 'react';
import { Card } from '@heroui/react/card';
import { Skeleton } from '@heroui/react/skeleton';
import { Logo, BoldWallet } from '@/shared/assets/icons';
import type { components } from '@/shared/api/generated';
import { useOperationsForProperties } from '../lib/use-operations-for-properties';
import { aggregateOperations } from '../lib/finance-aggregator';
import { formatMoney } from '../lib/format-money';
import { EmptyState } from '@/shared/ui/empty-state';
import { SectionHeader } from './SectionHeader';
import styles from './FinanceSection.module.css';

type PropertyResponse = components['schemas']['PropertyResponse'];

type FinanceSectionProps = {
  readonly properties: PropertyResponse[] | undefined;
  readonly isLoading: boolean;
};

const currentMonth = new Intl.DateTimeFormat('ru-RU', { month: 'long' }).format(new Date());

export function FinanceSection({ properties, isLoading }: FinanceSectionProps): JSX.Element {
  const { operationsList, isLoading: operationsLoading } = useOperationsForProperties(properties);
  const showLoading = isLoading || operationsLoading;

  const { actual, pending } = aggregateOperations(operationsList);
  const { incomeKopecks, expenseKopecks, profitKopecks } = actual;
  const hasPending = pending.incomeKopecks !== 0 || pending.expenseKopecks !== 0;

  if (showLoading) {
    return (
      <section className={styles.section}>
        <Skeleton className={styles.titleSkeleton} />
        <Card className={styles.card}>
          <div className={styles.topSkeleton}>
            <div className={styles.profitSkeleton}>
              <Skeleton className={styles.valueSkeleton} />
              <Skeleton className={styles.labelSkeleton} />
            </div>
            <Skeleton className={styles.logoSkeleton} />
          </div>
          <div className={styles.bottomSkeleton}>
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
        <SectionHeader title="Финансы и операции" href="/finance" />
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
      <SectionHeader title="Финансы и операции" href="/finance" />
      <Card className={styles.card}>
        <span className={styles.logo3d} aria-hidden="true" />
        <div className={styles.top}>
          <div className={styles.profit}>
            <span className={styles.profitValue}>{formatMoney(profitKopecks)}</span>
            <span className={styles.profitLabel}>Прибыль за {currentMonth}</span>
            {hasPending && (
              <div className={styles.pending}>
                <span className={styles.pendingLabel}>Ожидает оплаты</span>
                <span className={styles.pendingValue}>
                  {formatMoney(pending.incomeKopecks)} / {formatMoney(pending.expenseKopecks)}
                </span>
              </div>
            )}
          </div>
          <Logo className={styles.miniLogo} />
        </div>
        <div className={styles.bottom}>
          <div className={styles.column}>
            <span className={styles.value}>{formatMoney(incomeKopecks)}</span>
            <span className={styles.label}>Доходы</span>
          </div>
          <div className={styles.columnRight}>
            <span className={styles.value}>{formatMoney(expenseKopecks)}</span>
            <span className={styles.label}>Расходы</span>
          </div>
        </div>
      </Card>
    </section>
  );
}
