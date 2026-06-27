'use client';

import type { JSX } from 'react';
import NextLink from 'next/link';
import { Icon } from '@/shared/ui/icon';
import { LinkButton } from '@/shared/ui/link-button';
import { ArrowRight, UserSmall } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import type { components } from '@/shared/api/generated';
import { PropertyDetailSection } from './PropertyDetailSection';
import styles from './PropertyTenantCard.module.css';

type LeaseResponse = components['schemas']['LeaseResponse'];

export type PropertyTenantCardProps = {
  readonly lease: LeaseResponse | undefined;
  readonly propertyId: string;
};

export function PropertyTenantCard({
  lease,
  propertyId,
}: PropertyTenantCardProps): JSX.Element {
  const tenant = lease?.tenant_contact;
  const tenantsHref = `${ROUTES.tenants}?propertyId=${propertyId}`;

  return (
    <PropertyDetailSection>
      <div className={styles.header}>
        <h2 className={styles.title}>Арендатор</h2>
        <NextLink
          href={tenantsHref}
          className={styles.headerLink}
          aria-label="Перейти к арендаторам"
        >
          <Icon size="s">
            <ArrowRight />
          </Icon>
        </NextLink>
      </div>

      {tenant ? (
        <div className={styles.card}>
          <div className={styles.profile}>
            <span className={styles.avatar}>
              <Icon size="m">
                <UserSmall />
              </Icon>
            </span>
            <span className={styles.name}>
              {[tenant.name, tenant.surname, tenant.patronymic]
                .filter(Boolean)
                .join(' ')}
            </span>
          </div>
          {tenant.phone && (
            <p className={styles.field}>{tenant.phone}</p>
          )}
          {tenant.email && (
            <p className={styles.field}>{tenant.email}</p>
          )}
        </div>
      ) : (
        <div className={styles.empty}>
          <LinkButton href={tenantsHref} variant="primary" fullWidth>
            Добавить арендатора
          </LinkButton>
        </div>
      )}
    </PropertyDetailSection>
  );
}
