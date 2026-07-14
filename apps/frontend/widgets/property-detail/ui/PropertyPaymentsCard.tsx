'use client';

import type { JSX } from 'react';
import NextLink from 'next/link';
import { Icon } from '@/shared/ui/icon';
import { LinkButton } from '@/shared/ui/link-button';
import { ArrowRight } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import type { components } from '@/shared/api/generated';
import type { Property } from '@/entities/property/model/types';
import { OperationListItem } from '@/widgets/operations/ui/OperationListItem';
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
              <OperationListItem operation={operation} />
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
