'use client';

import type {JSX, ReactNode} from 'react';
import {Skeleton} from '@heroui/react/skeleton';
import {Button} from '@/shared/ui/button';
import type {OperationsFilters} from '@/features/operations/api/hooks';
import {useOperationsByProperty} from '@/features/operations/api/hooks';
import {OperationListItem} from '@/entities/operation/ui/OperationListItem';
import { DetailSection } from '@/shared/ui/detail-section';
import styles from './PropertyOperationsSection.module.css';
import {SectionHeader} from '@/shared/ui/section-header';

export type PropertyOperationsSectionProps = {
    readonly propertyId: string;
    readonly title: string;
    readonly href: string;
    readonly emptyText: string;
    readonly filters: Omit<OperationsFilters, 'property_id'>;
    readonly badge?: ReactNode;
};

export function PropertyOperationsSection({
                                              propertyId,
                                              title,
                                              href,
                                              emptyText,
                                              filters,
                                              badge,
                                          }: PropertyOperationsSectionProps): JSX.Element {
    const operationsQuery = useOperationsByProperty(propertyId, filters);

    const operations = operationsQuery.data?.items ?? [];

    return (
        <DetailSection>
            {badge}
            <SectionHeader title={title} href={href}/>

            {operationsQuery.isLoading && (
                <ul className={styles.list}>
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
            )}

            {!operationsQuery.isLoading && operationsQuery.isError && (
                <div className={styles.error} role="alert" aria-live="polite">
                    <p className={styles.errorText}>Не удалось загрузить операции</p>
                    <Button
                        variant="secondary"
                        size="small"
                        loading={operationsQuery.isFetching}
                        onClick={() => {
                            void operationsQuery.refetch();
                        }}
                    >
                        Повторить
                    </Button>
                </div>
            )}

            {!operationsQuery.isLoading && !operationsQuery.isError && operations.length === 0 && (
                <p className={styles.emptyText}>{emptyText}</p>
            )}

            {!operationsQuery.isLoading && !operationsQuery.isError && operations.length > 0 && (
                <ul className={styles.list}>
                    {operations.map((operation) => (
                        <li key={operation.id}>
                            <OperationListItem operation={operation}/>
                        </li>
                    ))}
                </ul>
            )}
        </DetailSection>
    );
}
