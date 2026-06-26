'use client';

import type { JSX } from 'react';
import { Icon } from '@/shared/ui/icon';
import { UserSmall } from '@/shared/assets/icons';
import { PropertyDetailSection } from '@/widgets/property-detail';
import { getTenantContactFullName } from '../lib/get-tenant-contact-full-name';
import type { TenantContact } from '@/entities/tenant-contact/model/types';
import styles from './TenantInfoSection.module.css';

export type TenantInfoSectionProps = {
  readonly tenant: TenantContact;
};

export function TenantInfoSection({ tenant }: TenantInfoSectionProps): JSX.Element {
  const fullName = getTenantContactFullName(tenant);

  return (
    <PropertyDetailSection>
      <h2 className={styles.title}>Контакты</h2>
      <div className={styles.card}>
        <div className={styles.profile}>
          <span className={styles.avatar}>
            <Icon size="m">
              <UserSmall />
            </Icon>
          </span>
          <span className={styles.name}>{fullName}</span>
        </div>
        {tenant.phone && <p className={styles.field}>{tenant.phone}</p>}
        {tenant.email && <p className={styles.field}>{tenant.email}</p>}
      </div>
    </PropertyDetailSection>
  );
}
