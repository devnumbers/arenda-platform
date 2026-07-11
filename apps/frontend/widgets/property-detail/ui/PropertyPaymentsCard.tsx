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
import type { Property } from '@/entities/property/model/types';
import { getCategoryLabel } from '@/entities/operation/lib/categories';
import { formatOverdueCount } from '../lib/format-overdue-count';
import { PropertyDetailSection } from './PropertyDetailSection';
import styles from './PropertyPaymentsCard.module.css';

type OperationResponse = components['schemas']['OperationResponse'];

export type PropertyPaymentsCardProps = {
  readonly property: Property;
  readonly operations: OperationResponse[];
  readonly overdueCount: number;
};

export function PropertyPaymentsCard({
  property,
  operations,
  overdueCount,
}: PropertyPaymentsCardProps): JSX.Element {
  const isArchived = property.status === 'archived';
  const recent = [...operations]
    .sort((a, b) => {
      const byDate = b.operation_date.localeCompare(a.operation_date);
      return byDate !== 0 ? byDate : b.created_at.localeCompare(a.created_at);
    })
    .slice(0, 3);

  return (
    <PropertyDetailSection>
      <div className={styles.header}>
        <h2 className={styles.title}>Операции</h2>
        <NextLink
          href={`${ROUTES.financeOperations}?property_id=${property.id}`}
          className={styles.headerLink}
          aria-label="Все операции"
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
            <li key={operation.id}>
              <NextLink
                href={ROUTES.financeOperation(operation.id)}
                className={styles.row}
              >
                <span className={styles.category}>
                  {getCategoryLabel(operation.category)}
                </span>
                <span className={styles.amount}>
                  {formatMoneyKopecks(operation.amount_kopecks)}
                </span>
                <span className={styles.date}>
                  {formatDate(operation.operation_date)}
                </span>
              </NextLink>
            </li>
          ))}
        </ul>
      ) : (
        <div className={styles.empty}>
          <p className={styles.emptyText}>Операций пока нет</p>
        </div>
      )}

      <div className={styles.actions}>
        <LinkButton
          href={`${ROUTES.financeCreateOperation}?propertyId=${property.id}`}
          variant="primary"
          fullWidth
          disabled={isArchived}
          title={isArchived ? 'Объект в архиве' : undefined}
        >
          Добавить операцию
        </LinkButton>
        {recent.length > 0 && (
          <LinkButton
            href={`${ROUTES.financeOperations}?property_id=${property.id}`}
            variant="secondary"
            fullWidth
          >
            Все операции
          </LinkButton>
        )}
      </div>
    </PropertyDetailSection>
  );
}
