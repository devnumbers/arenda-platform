'use client';

import type { JSX } from 'react';
import NextLink from 'next/link';
import { ROUTES } from '@/shared/config/routes';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import { formatLeaseRemainingDuration } from '@/shared/lib/format-lease-card-values';
import { useProperty } from '@/features/properties/api';
import { PropertyDetailSection } from '@/widgets/property-detail';
import { getLeaseStatusLabel } from '../lib/get-lease-status-label';
import type { Lease } from '@/entities/lease/model/types';
import styles from './TenantLeaseSection.module.css';

export type TenantLeaseSectionProps = {
  readonly lease: Lease;
};

export function TenantLeaseSection({ lease }: TenantLeaseSectionProps): JSX.Element {
  const propertyQuery = useProperty(lease.propertyId ?? '');
  const property = propertyQuery.data;
  const isPropertyError = propertyQuery.isError;
  const propertyHref = lease.propertyId ? ROUTES.property(lease.propertyId) : null;
  const remaining = formatLeaseRemainingDuration(lease.startDate, lease.endDate);

  const card = (
    <>
      <div className={styles.row}>
        <span className={styles.status}>{getLeaseStatusLabel(lease.status)}</span>
        <span className={styles.amount}>{formatMoneyKopecks(lease.rentKopecks)}</span>
      </div>

      <div className={styles.row}>
        <span className={styles.address}>
          {!lease.propertyId
            ? 'Без объекта'
            : isPropertyError
              ? 'Не удалось загрузить адрес'
              : (property?.address ?? 'Загрузка адреса…')}
        </span>
        <span className={styles.duration}>{remaining}</span>
      </div>
    </>
  );

  return (
    <PropertyDetailSection>
      <div className={styles.header}>
        <h2 className={styles.title}>Текущая аренда</h2>
      </div>

      {propertyHref ? (
        <NextLink href={propertyHref} className={styles.card}>
          {card}
        </NextLink>
      ) : (
        <div className={styles.card}>{card}</div>
      )}
    </PropertyDetailSection>
  );
}
