'use client';

import type { JSX } from 'react';
import NextLink from 'next/link';
import { ROUTES } from '@/shared/config/routes';
import { getTenantContactFullName } from '@/entities/tenant-contact/lib/get-tenant-contact-full-name';
import { getTenantSubtitle } from '../lib/get-tenant-subtitle';
import type { TenantContact } from '@/entities/tenant-contact/model/types';
import styles from './TenantCard.module.css';

export type TenantCardProps = {
  readonly tenant: TenantContact;
};

export function TenantCard({ tenant }: TenantCardProps): JSX.Element {
  const subtitle = getTenantSubtitle(tenant);
  const fullName = getTenantContactFullName(tenant);

  return (
    <NextLink
      href={ROUTES.tenant(tenant.id)}
      className={styles.root}
      aria-label={`Открыть арендатора ${fullName}`}
    >
      <span className={styles.name}>{fullName}</span>
      {subtitle && <span className={styles.subtitle}>{subtitle}</span>}
    </NextLink>
  );
}
