'use client';

import {type JSX, useMemo} from 'react';
import {Icon} from '@/shared/ui/icon';
import {PageHeader} from '@/shared/ui/page-header';
import {ArrowRight} from '@/shared/assets/icons';
import {LinkButton} from '@/shared/ui/link-button';
import {ROUTES} from '@/shared/config/routes';
import {useFinanceReport} from '@/features/finance';
import {useOperations} from '@/features/operations';
import {formatDateForApi} from '@/entities/operation';
import {FinanceCreateOperationButton} from './FinanceCreateOperationButton';
import {FinanceEmptyState} from '@/shared/ui/finance-empty-state';
import {FinanceLoading} from '@/shared/ui/finance-loading';
import {FinanceErrorState} from '@/shared/ui/finance-error-state';
import {FinanceSummaryCards} from './FinanceSummaryCards';
import {SubscriptionReadonlyBanner} from '@/features/subscription';
import {FinanceOperationsPreview, FinanceUpcomingOperations,} from './FinanceUpcomingOperations';
import {useSubscription} from '@/features/subscription';
import {isSubscriptionReadonly} from '@/features/subscription';
import styles from './FinancePage.module.css';
import sectionStyles from './FinanceSection.module.css';

function getUpcomingMonthRange(): { from: string; to: string } {
    const today = new Date();
    const until = new Date(today);
    until.setDate(today.getDate() + 30);
    return {from: formatDateForApi(today), to: formatDateForApi(until)};
}

export function FinancePage(): JSX.Element {
    const upcomingRange = useMemo(() => getUpcomingMonthRange(), []);

    const {
        data: report,
        isLoading: isReportLoading,
        isError: isReportError,
        isFetching: isReportFetching,
        refetch: refetchReport,
    } = useFinanceReport();

    const {
        data: operationsData,
        isLoading: isOperationsLoading,
        isError: isOperationsError,
        isFetching: isOperationsFetching,
        refetch: refetchOperations,
    } = useOperations({limit: 5});

    const {
        data: overdueOperationsData,
        isLoading: isLoadingOverdue,
        isError: isErrorOverdue,
        isFetching: isFetchingOverdue,
        refetch: refetchOverdue,
    } = useOperations({
        status: ['overdue'],
        sort: 'operation_date_asc',
        limit: 5,
        exclude_archived_properties: true,
    });

    const {
        data: upcomingOperationsData,
        isLoading: isLoadingUpcoming,
        isError: isErrorUpcoming,
        isFetching: isFetchingUpcoming,
        refetch: refetchUpcoming,
    } = useOperations({
        status: ['pending'],
        from: upcomingRange.from,
        to: upcomingRange.to,
        sort: 'operation_date_asc',
        limit: 5,
        exclude_archived_properties: true,
    });

    const isLoading = isReportLoading || isOperationsLoading;
    const isError = isReportError || isOperationsError;
    const isFetching = isReportFetching || isOperationsFetching;

    const handleRetry = () => {
        if (isReportError) {
            void refetchReport();
        }
        if (isOperationsError) {
            void refetchOperations();
        }
    };

    const operations = operationsData?.items ?? [];
    const overdueOperations = overdueOperationsData?.items ?? [];
    const upcomingOperations = upcomingOperationsData?.items ?? [];
    const shouldShowEmptyState =
        operations.length === 0 &&
        overdueOperations.length === 0 &&
        upcomingOperations.length === 0 &&
        !isLoadingOverdue &&
        !isLoadingUpcoming &&
        !isErrorOverdue &&
        !isErrorUpcoming;

    const {data: subscription, isPending: isSubscriptionPending} = useSubscription();
    const readonly = isSubscriptionPending || isSubscriptionReadonly(subscription);

    return (
        <div className={styles.root}>
            <PageHeader
                title="Финансы"
                actions={
                    !readonly && <FinanceCreateOperationButton/>
                }
            />

            <SubscriptionReadonlyBanner/>

            {isLoading && <FinanceLoading/>}

            {!isLoading && isError && <FinanceErrorState onRetry={handleRetry} isLoading={isFetching}/>}

            {!isLoading && !isError && (
                <>
                    {shouldShowEmptyState ? (
                        <FinanceEmptyState
                            title="Нет операций"
                            subtitle="Добавьте первую операцию, чтобы увидеть финансовую сводку"
                            actionHref={readonly ? undefined : ROUTES.financeCreateOperation}
                            actionText={readonly ? undefined : 'Добавить операцию'}
                        />
                    ) : (
                        <>
                            {report && (
                                <FinanceSummaryCards
                                    incomeKopecks={report.totals.income_kopecks}
                                    expenseKopecks={report.totals.expense_kopecks}
                                    profitKopecks={report.totals.profit_kopecks}
                                />
                            )}

                            <FinanceOperationsPreview
                                title="Просроченные операции"
                                operations={overdueOperations}
                                emptyText="Нет просроченных операций"
                                isLoading={isLoadingOverdue}
                                isFetching={isFetchingOverdue}
                                isError={isErrorOverdue}
                                refetch={refetchOverdue}
                                actionHref={`${ROUTES.financeOperations}?status=overdue&period=all&sort=operation_date_asc`}
                            />

                            <FinanceUpcomingOperations
                                operations={upcomingOperations}
                                isLoading={isLoadingUpcoming}
                                isFetching={isFetchingUpcoming}
                                isError={isErrorUpcoming}
                                refetch={refetchUpcoming}
                            />
                        </>
                    )}

                    {operations.length > 0 && (
                        <section className={sectionStyles.section}>
                            <h2 className={sectionStyles.sectionTitle}>Быстрые ссылки</h2>
                            <nav className={styles.quickLinks}>
                                <LinkButton
                                    href={ROUTES.financeOperations}
                                    variant="clear"
                                    size="medium"
                                    fullWidth
                                    rightIcon={
                                        <Icon size="s">
                                            <ArrowRight/>
                                        </Icon>
                                    }
                                    className={styles.quickLink}
                                >
                                    Все операции
                                </LinkButton>
                            </nav>
                        </section>
                    )}
                </>
            )}
        </div>
    );
}

