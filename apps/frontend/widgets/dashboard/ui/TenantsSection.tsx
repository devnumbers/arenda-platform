'use client';

import { useMemo, type JSX } from 'react';
import { Skeleton } from '@heroui/react/skeleton';
import { Icon } from '@/shared/ui/icon';
import { Arendators } from '@/shared/assets/icons';
import type { components } from '@/shared/api/generated';
import { EmptyState } from '@/shared/ui/empty-state';
import { SectionHeader } from './SectionHeader';
import { EntityCard } from './EntityCard';
import { IconActionCard } from './IconActionCard';
import styles from './TenantsSection.module.css';

type LeaseResponse = components['schemas']['LeaseResponse'];
type TenantContactResponse = components['schemas']['TenantContactResponse'];

type TenantsSectionProps = {
  readonly leases: LeaseResponse[] | undefined;
  readonly isLoading: boolean;
};

export function TenantsSection({ leases, isLoading }: TenantsSectionProps): JSX.Element {
  const tenants = useMemo(() => {
    const seen = new Map<string, TenantContactResponse>();
    for (const lease of leases ?? []) {
      const contact = lease.tenant_contact;
      if (contact && !seen.has(contact.id)) {
        seen.set(contact.id, contact);
      }
    }
    return Array.from(seen.values());
  }, [leases]);
  const count = tenants.length;

  if (isLoading) {
    return (
      <section className={styles.section}>
        <Skeleton className={styles.titleSkeleton} />
        <div className={styles.scroll}>
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
        <SectionHeader title="Арендаторы" href="/tenants" />
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
      <SectionHeader title="Арендаторы" count={count} href="/tenants" />
      <div className={styles.scroll}>
        {tenants.map((tenant) => (
          <EntityCard
            key={tenant.id}
            href={`/tenants/${tenant.id}`}
            title={tenant.name}
            subtitle={tenant.surname ?? undefined}
          />
        ))}
        <IconActionCard
          href="/tenants"
          icon={
            <Icon size="l">
              <Arendators />
            </Icon>
          }
          label="Все"
          variant="outlined"
          className={styles.allCard}
        />
      </div>
    </section>
  );
}
