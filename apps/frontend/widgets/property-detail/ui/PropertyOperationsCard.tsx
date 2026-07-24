'use client';

import type {JSX} from 'react';
import {Card} from '@heroui/react/card';
import {formatMoneyKopecks} from '@/shared/lib/format-money';
import {ROUTES} from '@/shared/config/routes';
import type {components} from '@/shared/api/generated';
import {SectionHeader} from '@/widgets/dashboard/ui/SectionHeader';
import {PropertyDetailSection} from './PropertyDetailSection';
import styles from './PropertyOperationsCard.module.css';

type PropertyOperationsSummaryResponse =
    components['schemas']['PropertyOperationsSummaryResponse'];

export type PropertyOperationsCardProps = {
    readonly propertyId: string;
    readonly summary: PropertyOperationsSummaryResponse | undefined;
};

export function PropertyOperationsCard({
                                           propertyId,
                                           summary,
                                       }: PropertyOperationsCardProps): JSX.Element {
    const income = summary?.all_time_income_kopecks ?? 0;
    const expense = summary?.all_time_expense_kopecks ?? 0;
    const isEmpty = !summary || (income === 0 && expense === 0);

    return (
        <PropertyDetailSection>
            <SectionHeader
                title="Операции объекта"
                href={`${ROUTES.financeOperations}?property_id=${propertyId}&period=all`}
            />

            {isEmpty ? (
                <p className={styles.emptyText}>
                    Операций ещё не было. Здесь будет отображаться прибыль по объекту.
                </p>
            ) : (
                <Card className={styles.card}>
                    <span className={styles.logo3d} aria-hidden="true"/>
                    <div className={styles.top}>
                        <div className={styles.profit}>
              <span className={styles.profitValue}>
                {formatMoneyKopecks(summary.all_time_profit_kopecks, {round: true})}
              </span>
                            <span className={styles.profitLabel}>Прибыль за всё время</span>
                        </div>
                    </div>
                    <div className={styles.bottom}>
                        <div className={styles.column}>
              <span className={styles.value}>
                {formatMoneyKopecks(income, {round: true})}
              </span>
                            <span className={styles.label}>Доходы за всё время</span>
                        </div>
                        <div className={styles.columnRight}>
              <span className={styles.value}>
                {formatMoneyKopecks(expense, {round: true})}
              </span>
                            <span className={styles.label}>Расходы за всё время</span>
                        </div>
                    </div>
                </Card>
            )}
        </PropertyDetailSection>
    );
}
