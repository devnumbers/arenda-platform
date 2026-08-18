'use client';

import {type JSX, useMemo} from 'react';
import {Skeleton} from '@heroui/react/skeleton';
import {ROUTES} from '@/shared/config/routes';
import {SectionHeader} from '@/shared/ui/section-header';
import {OperationListItem} from '@/entities/operation';
import type {Operation} from '@/entities/operation';
import {FinanceErrorState} from '@/shared/ui/finance-error-state';
import sectionStyles from './FinanceSection.module.css';
import styles from './FinanceUpcomingOperations.module.css';

interface FinanceOperationsPreviewProps {
    readonly title: string;
    readonly operations: ReadonlyArray<Operation>;
    readonly emptyText: string;
    readonly isLoading: boolean;
    readonly isFetching: boolean;
    readonly isError: boolean;
    readonly refetch: () => void;
    readonly actionHref: string;
}

export function FinanceOperationsPreview({
                                             title,
                                             operations: rawOperations,
                                             emptyText,
                                             isLoading,
                                             isFetching,
                                             isError,
                                             refetch,
                                             actionHref,
                                         }: FinanceOperationsPreviewProps): JSX.Element {
    const operations = useMemo(() => {
        return [...rawOperations].sort(
            (a, b) => new Date(a.operationDate).getTime() - new Date(b.operationDate).getTime(),
        );
    }, [rawOperations]);

    if (isLoading) {
        return (
            <section className={sectionStyles.section}>
                <SectionHeader title={title} href={actionHref}/>
                <ul className={sectionStyles.operationsList}>
                    <li>
                        <Skeleton className={styles.rowSkeleton}/>
                    </li>
                    <li>
                        <Skeleton className={styles.rowSkeleton}/>
                    </li>
                    <li>
                        <Skeleton className={styles.rowSkeleton}/>
                    </li>
                </ul>
            </section>
        );
    }

    if (isError) {
        return (
            <section className={sectionStyles.section}>
                <SectionHeader title={title} href={actionHref}/>
                <FinanceErrorState onRetry={refetch} isLoading={isFetching}/>
            </section>
        );
    }

    return (
        <section className={sectionStyles.section}>
            <SectionHeader title={title} href={actionHref}/>
            {operations.length === 0 ? (
                <p className={styles.upcomingEmpty}>{emptyText}</p>
            ) : (
                <ul className={sectionStyles.operationsList}>
                    {operations.map((operation) => (
                        <li key={operation.id}>
                            <OperationListItem operation={operation}/>
                        </li>
                    ))}
                </ul>
            )}
        </section>
    );
}

export function FinanceUpcomingOperations(
    props: Omit<FinanceOperationsPreviewProps, 'title' | 'emptyText' | 'actionHref'>,
): JSX.Element {
    return (
        <FinanceOperationsPreview
            {...props}
            title="Ближайшие операции"
            emptyText="Нет запланированных операций"
            actionHref={`${ROUTES.financeOperations}?status=pending`}
        />
    );
}
