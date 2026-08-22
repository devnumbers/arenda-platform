'use client';

import { useMemo, useState, type JSX } from 'react';
import { Button } from '@/shared/ui/button';
import {
  formatDateForApi,
  startOfMonth,
  endOfMonth,
} from '@/entities/operation';
import { useFinanceReport } from '@/features/finance';
import { useProperties } from '@/features/properties';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import { FinanceLoading } from '@/shared/ui/finance-loading';
import { FinanceErrorState } from '@/shared/ui/finance-error-state';
import { FinanceEmptyState } from '@/shared/ui/finance-empty-state';
import type { OperationType } from '@/entities/operation';
import styles from './ProfitReport.module.css';

type Period = 'month' | 'quarter' | 'year';

function getCurrentMonthRange(): { from: string; to: string } {
  const now = new Date();
  return {
    from: formatDateForApi(startOfMonth(now)),
    to: formatDateForApi(endOfMonth(now)),
  };
}

function getQuarterRange(date: Date): { from: string; to: string } {
  const quarter = Math.floor(date.getMonth() / 3);
  const from = new Date(date.getFullYear(), quarter * 3, 1);
  const to = new Date(date.getFullYear(), quarter * 3 + 3, 0);
  return { from: formatDateForApi(from), to: formatDateForApi(to) };
}

function getYearRange(date: Date): { from: string; to: string } {
  const from = new Date(date.getFullYear(), 0, 1);
  const to = new Date(date.getFullYear(), 11, 31);
  return { from: formatDateForApi(from), to: formatDateForApi(to) };
}

function getRangeForPeriod(period: Period): { from: string; to: string } {
  const now = new Date();
  if (period === 'month') {
    return getCurrentMonthRange();
  }
  if (period === 'quarter') {
    return getQuarterRange(now);
  }
  return getYearRange(now);
}

function getOperationTypeLabel(type: OperationType): string {
  return type === 'income' ? 'Доход' : 'Расход';
}

function formatMonthLabel(month: string): string {
  return new Date(month).toLocaleDateString('ru-RU', {
    month: 'short',
    year: 'numeric',
  });
}

const PERIOD_BUTTONS: ReadonlyArray<{ key: Period; label: string }> = [
  { key: 'month', label: 'Месяц' },
  { key: 'quarter', label: 'Квартал' },
  { key: 'year', label: 'Год' },
];

export function ProfitReport(): JSX.Element {
  const [period, setPeriod] = useState<Period>('month');
  const { from, to } = useMemo(() => getRangeForPeriod(period), [period]);

  const {
    data: report,
    isLoading,
    isFetching,
    isError,
    refetch,
  } = useFinanceReport(from, to);

  const { data: properties } = useProperties();

  const propertyById = useMemo(() => {
    const map = new Map<string, string>();
    properties?.forEach((property) => {
      map.set(property.id, property.name);
    });
    return map;
  }, [properties]);

  if (isLoading) {
    return <FinanceLoading />;
  }

  if (isError) {
    return <FinanceErrorState onRetry={() => void refetch()} isLoading={isFetching} />;
  }

  const totals = report?.totals;
  const isEmpty =
    !totals ||
    (totals.income_kopecks === 0 &&
      totals.expense_kopecks === 0 &&
      totals.profit_kopecks === 0);

  if (isEmpty) {
    return (
      <FinanceEmptyState
        title="Нет данных за период"
        subtitle="Операции за выбранный период отсутствуют."
      />
    );
  }

  const maxIncome = Math.max(
    ...(report.by_month?.map((row) => row.income_kopecks) ?? [0]),
  );

  return (
    <div className={styles.root}>
      <div className={styles.periodRow}>
        {PERIOD_BUTTONS.map((button) => (
          <Button
            key={button.key}
            variant={period === button.key ? 'secondary' : 'icon-black'}
            size="small"
            onClick={() => setPeriod(button.key)}
          >
            {button.label}
          </Button>
        ))}
      </div>

      <div className={styles.totals}>
        <div className={styles.totalCard}>
          <span className={styles.totalLabel}>Доход</span>
          <span className={`${styles.totalValue} ${styles.income}`}>
            {formatMoneyKopecks(totals.income_kopecks, { round: true })}
          </span>
        </div>
        <div className={styles.totalCard}>
          <span className={styles.totalLabel}>Расход</span>
          <span className={`${styles.totalValue} ${styles.expense}`}>
            {formatMoneyKopecks(totals.expense_kopecks, { round: true })}
          </span>
        </div>
        <div className={styles.totalCard}>
          <span className={styles.totalLabel}>Прибыль</span>
          <span className={styles.totalValue}>
            {formatMoneyKopecks(totals.profit_kopecks, { round: true })}
          </span>
        </div>
      </div>

      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>По объектам</h2>
        <ul className={styles.rows}>
          {report.by_property?.map((row) => (
            <li key={row.property_id ?? 'no-property'} className={styles.row}>
              <span className={styles.rowName}>
                {row.property_id
                  ? (propertyById.get(row.property_id) ?? row.property_id)
                  : 'Без объекта'}
              </span>
              <div className={styles.rowAmounts}>
                <span className={styles.incomeAmount}>
                  {formatMoneyKopecks(row.income_kopecks, { round: true })}
                </span>
                <span className={styles.expenseAmount}>
                  {formatMoneyKopecks(row.expense_kopecks, { round: true })}
                </span>
                <span className={styles.profitAmount}>
                  {formatMoneyKopecks(row.profit_kopecks, { round: true })}
                </span>
              </div>
            </li>
          ))}
        </ul>
      </section>

      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>По категориям</h2>
        <ul className={styles.rows}>
          {report.by_category?.map((row, index) => (
            <li key={`${row.type}-${row.category_id}-${index}`} className={styles.row}>
              <div className={styles.rowMain}>
                <span className={styles.rowName}>
                  {row.category_name}
                </span>
                <span className={styles.rowMeta}>
                  {getOperationTypeLabel(row.type)}
                </span>
              </div>
              <span className={styles.rowAmount}>
                {formatMoneyKopecks(row.total_kopecks, { round: true })}
              </span>
            </li>
          ))}
        </ul>
      </section>

      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>По месяцам</h2>
        <div className={styles.months}>
          {report.by_month?.map((row) => {
            const widthPercent =
              maxIncome > 0 ? (row.income_kopecks / maxIncome) * 100 : 0;
            return (
              <div key={row.month} className={styles.monthRow}>
                <div className={styles.monthHeader}>
                  <span className={styles.monthLabel}>{formatMonthLabel(row.month)}</span>
                  <div className={styles.monthValues}>
                    <span className={styles.incomeAmount}>
                      {formatMoneyKopecks(row.income_kopecks, { round: true })}
                    </span>
                    <span className={styles.expenseAmount}>
                      {formatMoneyKopecks(row.expense_kopecks, { round: true })}
                    </span>
                    <span className={styles.profitAmount}>
                      {formatMoneyKopecks(row.profit_kopecks, { round: true })}
                    </span>
                  </div>
                </div>
                <div className={styles.barTrack}>
                  <div
                    className={styles.bar}
                    style={{ width: `${widthPercent}%` }}
                    aria-hidden="true"
                  />
                </div>
              </div>
            );
          })}
        </div>
      </section>
    </div>
  );
}
