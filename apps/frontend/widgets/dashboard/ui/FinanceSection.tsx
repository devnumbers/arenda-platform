'use client';

import type {JSX} from 'react';
import NextLink from 'next/link';
import {Card} from '@heroui/react/card';
import {Skeleton} from '@heroui/react/skeleton';
import {BoldWallet} from '@/shared/assets/icons';
import type {Property} from '@/entities/property/model/types';
import {useOperationsForProperties} from '../lib/use-operations-for-properties';
import {aggregateOperations} from '../lib/finance-aggregator';
import {formatMoneyKopecks} from '@/shared/lib/format-money';
import {EmptyState} from '@/shared/ui/empty-state';
import {SectionHeader} from './SectionHeader';
import styles from './FinanceSection.module.css';

type FinanceSectionProps = {
    readonly properties: Property[] | undefined;
    readonly isLoading: boolean;
};

const currentMonth = new Intl.DateTimeFormat('ru-RU', {month: 'long'}).format(new Date());

export function FinanceSection({properties, isLoading}: FinanceSectionProps): JSX.Element {
    const {operationsList, isLoading: operationsLoading} = useOperationsForProperties(properties);
    const showLoading = isLoading || operationsLoading;

    const now = new Date();
    const currentYM = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`;
    const monthOperationsList = operationsList.map((response) => ({
        ...response,
        items: response.items.filter((op) => op.operation_date.slice(0, 7) === currentYM),
    }));
    const {actual} = aggregateOperations(monthOperationsList);
    const {incomeKopecks, expenseKopecks, profitKopecks} = actual;

    if (showLoading) {
        return (
            <section className={styles.section}>
                <Skeleton className={styles.titleSkeleton}/>
                <Card className={styles.card}>
                    <div className={styles.topSkeleton}>
                        <div className={styles.profitSkeleton}>
                            <Skeleton className={styles.valueSkeleton}/>
                            <Skeleton className={styles.labelSkeleton}/>
                        </div>
                        <Skeleton className={styles.logoSkeleton}/>
                    </div>
                    <div className={styles.bottomSkeleton}>
                        <Skeleton className={styles.columnSkeleton}/>
                        <Skeleton className={styles.columnSkeleton}/>
                    </div>
                </Card>
            </section>
        );
    }

    if (operationsList.length === 0) {
        return (
            <section className={styles.section}>
                <SectionHeader title="Финансы и операции" href="/finance"/>
                <EmptyState
                    icon={<BoldWallet/>}
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
            <SectionHeader title="Финансы и операции" href="/finance"/>
            <NextLink href="/finance" className={styles.cardLink}>
                <Card className={styles.card}>
                    <span className={styles.logo3d} aria-hidden="true"/>
                    <div className={styles.top}>
                        <div className={styles.profit}>
                            <span className={styles.profitValue}>{formatMoneyKopecks(profitKopecks, {round: true})}</span>
                            <span className={styles.profitLabel}>Прибыль за {currentMonth}</span>
                        </div>
                    </div>
                    <div className={styles.bottom}>
                        <div className={styles.column}>
                            <span className={styles.value}>{formatMoneyKopecks(incomeKopecks, {round: true})}</span>
                            <span className={styles.label}>Доходы</span>
                        </div>
                        <div className={styles.columnRight}>
                            <span className={styles.value}>{formatMoneyKopecks(expenseKopecks, {round: true})}</span>
                            <span className={styles.label}>Расходы</span>
                        </div>
                    </div>
                </Card>
            </NextLink>
        </section>
    );
}
