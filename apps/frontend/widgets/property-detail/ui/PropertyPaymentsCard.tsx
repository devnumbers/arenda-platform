'use client';

import type { JSX } from 'react';
import NextLink from 'next/link';
import { Icon } from '@/shared/ui/icon';
import { LinkButton } from '@/shared/ui/link-button';
import { ArrowRight } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import { formatDate } from '@/shared/lib/format-date';
import type { components } from '@/shared/api/generated';
import { formatOverdueCount } from '../lib/format-overdue-count';
import { PropertyDetailSection } from './PropertyDetailSection';
import styles from './PropertyPaymentsCard.module.css';

type OperationResponse = components['schemas']['OperationResponse'];

export type PropertyPaymentsCardProps = {
  readonly operations: OperationResponse[];
  readonly overdueCount: number;
};

const categoryLabels: Record<OperationResponse['category'], string> = {
  rent: 'Аренда',
  other_income: 'Другой доход',
  utilities: 'Коммунальные услуги',
  repair: 'Ремонт',
  tax: 'Налог',
  other_expense: 'Другой расход',
  deposit_return: 'Возврат залога',
};

export function PropertyPaymentsCard({
  operations,
  overdueCount,
}: PropertyPaymentsCardProps): JSX.Element {
  const recent = [...operations]
    .sort((a, b) => {
      const byDate = b.operation_date.localeCompare(a.operation_date);
      return byDate !== 0 ? byDate : b.created_at.localeCompare(a.created_at);
    })
    .slice(0, 3);

  return (
    <PropertyDetailSection>
      <div className={styles.header}>
        <h2 className={styles.title}>Платежи</h2>
        <NextLink
          href={ROUTES.finance}
          className={styles.headerLink}
          aria-label="Перейти к платежам"
        >
          <Icon size="s">
            <ArrowRight />
          </Icon>
        </NextLink>
      </div>

      {overdueCount > 0 && (
        <span className={styles.overdueBadge}>
          {formatOverdueCount(overdueCount)}
        </span>
      )}

      {recent.length > 0 ? (
        <ul className={styles.list}>
          {recent.map((operation) => (
            <li key={operation.id} className={styles.row}>
              <span className={styles.category}>
                {categoryLabels[operation.category]}
              </span>
              <span className={styles.amount}>
                {formatMoneyKopecks(operation.amount_kopecks)}
              </span>
              <span className={styles.date}>
                {formatDate(operation.operation_date)}
              </span>
            </li>
          ))}
        </ul>
      ) : (
        <div className={styles.empty}>
          <p className={styles.emptyText}>Платежей пока нет</p>
        </div>
      )}

      <div className={styles.actions}>
        {recent.length > 0 ? (
          <>
            <LinkButton href={ROUTES.finance} variant="primary" fullWidth>
              Внести платёж
            </LinkButton>
            <LinkButton href={ROUTES.finance} variant="secondary" fullWidth>
              Запланировать
            </LinkButton>
          </>
        ) : (
          <LinkButton href={ROUTES.finance} variant="primary" fullWidth>
            Добавить платежи
          </LinkButton>
        )}
      </div>
    </PropertyDetailSection>
  );
}
