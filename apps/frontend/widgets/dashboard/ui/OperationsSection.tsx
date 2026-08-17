'use client';

import {type JSX} from 'react';
import {Card} from '@heroui/react/card';
import {Skeleton} from '@heroui/react/skeleton';
import {type OperationsFilters, useOperations} from '@/features/operations/api/hooks';
import {SectionHeader} from '@/shared/ui/section-header';
import {OperationListItem} from '@/entities/operation/ui/OperationListItem';
import {FinanceErrorState} from '@/shared/ui/finance-error-state';
import styles from './OperationsSection.module.css';

export type OperationsSectionProps = {
    readonly title: string;
    readonly href: string;
    readonly filters: OperationsFilters;
};

export function OperationsSection({title, href, filters}: OperationsSectionProps): JSX.Element | null {
    const {
        data,
        isLoading,
        isFetching,
        isError,
        refetch,
    } = useOperations(filters);

    if (isLoading) {
        return (
            <section className={styles.section}>
                <Skeleton className={styles.titleSkeleton}/>
                <Card className={styles.card}>
                    <Skeleton className={styles.rowSkeleton}/>
                    <Skeleton className={styles.rowSkeleton}/>
                    <Skeleton className={styles.rowSkeleton}/>
                </Card>
            </section>
        );
    }

    if (isError) {
        return (
            <section className={styles.section}>
                <SectionHeader title={title} href={href}/>
                <FinanceErrorState onRetry={refetch} isLoading={isFetching}/>
            </section>
        );
    }

    const operations = data?.items ?? [];

    if (operations.length === 0) {
        return null;
    }

    return (
        <section className={styles.section}>
            <SectionHeader title={title} href={href}/>
            <Card className={styles.card}>
                <ul className={styles.list}>
                    {operations.map((operation) => (
                        <li key={operation.id}>
                            <OperationListItem operation={operation}/>
                        </li>
                    ))}
                </ul>
            </Card>
        </section>
    );
}
