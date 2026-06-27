'use client';

import { useMemo, type JSX } from 'react';
import { Icon } from '@/shared/ui/icon';
import { Plus, ArrowRight } from '@/shared/assets/icons';
import { LinkButton } from '@/shared/ui/link-button';
import { ROUTES } from '@/shared/config/routes';
import { useFinanceReport } from '@/features/finance/api/hooks';
import { useOperations } from '@/features/operations/api/hooks';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import { categoryLabels } from '../lib/category-labels';
import { statusLabels, statusVariants } from '../lib/status-labels';
import { FinanceEmptyState } from './FinanceEmptyState';
import { FinanceLoading } from './FinanceLoading';
import { FinanceErrorState } from './FinanceErrorState';
import { FinanceSummaryCards } from './FinanceSummaryCards';
import styles from './FinancePage.module.css';

function formatDateForApi(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

function getCurrentMonthRange(): { from: string; to: string } {
  const now = new Date();
  const from = new Date(now.getFullYear(), now.getMonth(), 1);
  const to = new Date(now.getFullYear(), now.getMonth() + 1, 0);
  return { from: formatDateForApi(from), to: formatDateForApi(to) };
}

export function FinancePage(): JSX.Element {
  const { from, to } = useMemo(() => getCurrentMonthRange(), []);

  const {
    data: report,
    isLoading: isReportLoading,
    isError: isReportError,
    isFetching: isReportFetching,
    refetch: refetchReport,
  } = useFinanceReport(from, to);

  const {
    data: operationsData,
    isLoading: isOperationsLoading,
    isError: isOperationsError,
    isFetching: isOperationsFetching,
    refetch: refetchOperations,
  } = useOperations({ limit: 5 });

  const isLoading = isReportLoading || isOperationsLoading;
  const isError = isReportError || isOperationsError;
  const isFetching = isReportFetching || isOperationsFetching;

  const handleRetry = () => {
    if (isReportError) {
      refetchReport();
    }
    if (isOperationsError) {
      refetchOperations();
    }
  };

  const operations = operationsData?.items ?? [];

  return (
    <div className={styles.root}>
      <div className={styles.header}>
        <h1 className={styles.title}>Финансы</h1>
        <LinkButton
          href={ROUTES.financeCreateOperation}
          variant="primary"
          size="medium"
          leftIcon={
            <Icon size="s">
              <Plus />
            </Icon>
          }
        >
          Добавить операцию
        </LinkButton>
      </div>

      {isLoading && <FinanceLoading />}

      {!isLoading && isError && <FinanceErrorState onRetry={handleRetry} isLoading={isFetching} />}

      {!isLoading && !isError && operations.length === 0 && (
        <FinanceEmptyState
          title="Нет операций"
          subtitle="Добавьте первую операцию, чтобы увидеть финансовую сводку"
          actionHref={ROUTES.financeCreateOperation}
          actionText="Добавить операцию"
        />
      )}

      {!isLoading && !isError && operations.length > 0 && (
        <>
          {report && (
            <FinanceSummaryCards
              incomeKopecks={report.totals.income_kopecks}
              expenseKopecks={report.totals.expense_kopecks}
              profitKopecks={report.totals.profit_kopecks}
            />
          )}

          <section className={styles.section}>
            <div className={styles.sectionHeader}>
              <h2 className={styles.sectionTitle}>Последние операции</h2>
              <LinkButton
                href={ROUTES.financeOperations}
                variant="clear"
                size="small"
                rightIcon={
                  <Icon size="s">
                    <ArrowRight />
                  </Icon>
                }
                className={styles.linkAll}
              >
                Смотреть все
              </LinkButton>
            </div>

            <ul className={styles.operationsList}>
              {operations.map((operation) => {
                const isIncome = operation.type === 'income';
                const amountClass = isIncome ? styles.amountIncome : styles.amountExpense;
                const sign = isIncome ? '+' : '-';
                const statusVariant = statusVariants[operation.status];

                return (
                  <li key={operation.id} className={styles.operationRow}>
                    <div className={styles.operationMain}>
                      <span className={styles.operationName}>{operation.name}</span>
                      <span className={styles.operationMeta}>
                        {new Date(operation.operation_date).toLocaleDateString('ru-RU')}
                        {' · '}
                        {categoryLabels[operation.category] ?? operation.category}
                      </span>
                    </div>
                    <div className={styles.operationRight}>
                      <span className={`${styles.operationAmount} ${amountClass}`}>
                        {sign}
                        {formatMoneyKopecks(operation.amount_kopecks, { round: true })}
                      </span>
                      <span
                        className={`${styles.statusBadge} ${
                          statusVariant === 'warning' ? styles.statusWarning : styles.statusSuccess
                        }`}
                      >
                        {statusLabels[operation.status] ?? operation.status}
                      </span>
                    </div>
                  </li>
                );
              })}
            </ul>
          </section>

          <section className={styles.section}>
            <h2 className={styles.sectionTitle}>Быстрые ссылки</h2>
            <nav className={styles.quickLinks}>
              <LinkButton
                href={ROUTES.financeOperations}
                variant="clear"
                size="medium"
                fullWidth
                rightIcon={
                  <Icon size="s">
                    <ArrowRight />
                  </Icon>
                }
                className={styles.quickLink}
              >
                Все операции
              </LinkButton>
              <LinkButton
                href={ROUTES.financePayments}
                variant="clear"
                size="medium"
                fullWidth
                rightIcon={
                  <Icon size="s">
                    <ArrowRight />
                  </Icon>
                }
                className={styles.quickLink}
              >
                Платежи
              </LinkButton>
              <LinkButton
                href={`${ROUTES.finance}?tab=report`}
                variant="clear"
                size="medium"
                fullWidth
                rightIcon={
                  <Icon size="s">
                    <ArrowRight />
                  </Icon>
                }
                className={styles.quickLink}
              >
                Отчёт о прибыли
              </LinkButton>
            </nav>
          </section>
        </>
      )}
    </div>
  );
}
