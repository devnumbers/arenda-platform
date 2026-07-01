'use client';

import {type JSX, useMemo} from 'react';
import {Card} from '@heroui/react/card';
import {Skeleton} from '@heroui/react/skeleton';
import {Clock} from '@/shared/assets/icons';
import {type OperationsFilters, useOperations} from '@/features/operations/api/hooks';
import {formatDateForApi} from '@/entities/operation/lib/dates';
import {ROUTES} from '@/shared/config/routes';
import {EmptyState} from '@/shared/ui/empty-state';
import {SectionHeader} from './SectionHeader';
import {UpcomingOperationRow} from './UpcomingOperationRow';
import {FinanceErrorState} from '@/widgets/finance/ui/FinanceErrorState';
import {useSubscription} from '@/features/subscription/api/hooks';
import {isSubscriptionReadonly} from '@/features/subscription/lib/is-subscription-readonly';
import styles from './UpcomingOperationsSection.module.css';

export function UpcomingOperationsSection(): JSX.Element {
    const filters: OperationsFilters = useMemo(
        () => {
            const today = new Date();
            const until = new Date(today);
            until.setDate(today.getDate() + 30);

            return {
                status: ['pending'],
                from: formatDateForApi(today),
                to: formatDateForApi(until),
                limit: 5,
            };
        },
        [],
    );

    const {
        data,
        isLoading,
        isFetching,
        isError,
        refetch,
    } = useOperations(filters);

    const operations = useMemo(() => {
        const items = data?.items ?? [];
        return [...items].sort(
            (a, b) => new Date(a.operation_date).getTime() - new Date(b.operation_date).getTime(),
        );
    }, [data]);

    const {data: subscription} = useSubscription();
    const readonly = isSubscriptionReadonly(subscription);

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
                <SectionHeader title="Ближайшие операции" href={ROUTES.finance}/>
                <FinanceErrorState onRetry={refetch} isLoading={isFetching}/>
            </section>
        );
    }

    if (operations.length === 0) {
        return (
            <section className={styles.section}>
                <SectionHeader title="Ближайшие операции" href={ROUTES.finance}/>
                <EmptyState
                    icon={<Clock/>}
                    title="Нет запланированных операций"
                    subtitle="Добавьте операцию, чтобы видеть её здесь"
                    actionHref={readonly ? undefined : ROUTES.financeCreateOperation}
                    actionText={readonly ? undefined : 'Добавить операцию'}
                />
            </section>
        );
    }

    return (
        <section className={styles.section}>
            <SectionHeader title="Ближайшие операции" href={ROUTES.finance}/>
            <Card className={styles.card}>
                <ul className={styles.list}>
                    {operations.map((operation) => (
                        <li key={operation.id}>
                            <UpcomingOperationRow operation={operation}/>
                        </li>
                    ))}
                </ul>
            </Card>
        </section>
    );
}
