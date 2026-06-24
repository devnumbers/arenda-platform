'use client';

import type { JSX } from 'react';
import NextLink from 'next/link';
import { Card } from '@heroui/react/card';
import { Skeleton } from '@heroui/react/skeleton';
import { Icon } from '@/shared/ui/icon';
import { Arendators } from '@/shared/assets/icons';
import type { components } from '@/shared/api/generated';
import { EmptyState } from './EmptyState';
import styles from './TenantsSection.module.css';

type LeaseResponse = components['schemas']['LeaseResponse'];
type TenantContactResponse = components['schemas']['TenantContactResponse'];

type TenantsSectionProps = {
  readonly leases: LeaseResponse[] | undefined;
  readonly isLoading: boolean;
};

export function TenantsSection({
  leases,
  isLoading,
}: TenantsSectionProps): JSX.Element {
  const tenants =
    leases
      ?.map((lease) => lease.tenant_contact)
      .filter(
        (contact): contact is TenantContactResponse => contact !== null,
      ) ?? [];
  const count = tenants.length;

  if (isLoading) {
    return (
      <section className={styles.section}>
        <Skeleton className={styles.titleSkeleton} />
        <div className={styles.list}>
          <Skeleton className={styles.itemSkeleton} />
          <Skeleton className={styles.itemSkeleton} />
          <Skeleton className={styles.itemSkeleton} />
        </div>
      </section>
    );
  }

  if (tenants.length === 0) {
    return (
      <section className={styles.section}>
        <h2 className={styles.title}>Арендаторы</h2>
        <EmptyState
          icon={<Arendators />}
          entities="арендаторов"
          subtitle="Добавьте арендатора в разделе аренды"
          actionHref="/tenants"
          actionText="Добавить арендатора"
        />
      </section>
    );
  }

  return (
    <section className={styles.section}>
      <div className={styles.header}>
        <h2 className={styles.title}>Арендаторы {count}</h2>
        <NextLink href="/tenants" className={styles.link}>
          Все
        </NextLink>
      </div>
      <div className={styles.scroll}>
        {tenants.map((tenant) => (
          <Card key={tenant.id} className={styles.card}>
            <div className={styles.placeholder} />
            <span className={styles.name}>
              {[tenant.name, tenant.surname].filter(Boolean).join(' ')}
            </span>
          </Card>
        ))}
        <NextLink href="/tenants" className={styles.allCard}>
          <Icon size="m">
            <Arendators />
          </Icon>
          <span>Все</span>
        </NextLink>
      </div>
    </section>
  );
}
