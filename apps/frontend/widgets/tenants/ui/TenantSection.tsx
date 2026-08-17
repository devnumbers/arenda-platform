'use client';

import type { JSX } from 'react';
import { TenantCard } from './TenantCard';
import type { TenantContact } from '@/entities/tenant-contact';
import styles from './TenantSection.module.css';

export type TenantSectionProps = {
  readonly title: string;
  readonly tenants: TenantContact[];
};

export function TenantSection({ title, tenants }: TenantSectionProps): JSX.Element | null {
  if (tenants.length === 0) {
    return null;
  }

  return (
    <section className={styles.root}>
      <h2 className={styles.title}>{title}</h2>
      <div className={styles.list}>
        {tenants.map((tenant) => (
          <TenantCard key={tenant.id} tenant={tenant} />
        ))}
      </div>
    </section>
  );
}
