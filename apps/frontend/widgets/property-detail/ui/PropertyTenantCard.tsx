'use client';

import type {JSX} from 'react';
import NextLink from 'next/link';
import {Icon} from '@/shared/ui/icon';
import {ArrowRight, UserSmall} from '@/shared/assets/icons';
import {ROUTES} from '@/shared/config/routes';
import {RETURN_TO_PARAM} from '@/shared/lib/navigation';
import type {Lease} from '@/entities/lease/model/types';
import {PropertyDetailSection} from './PropertyDetailSection';
import styles from './PropertyTenantCard.module.css';
import {SectionHeader} from "@/widgets/dashboard/ui/SectionHeader";

export type PropertyTenantCardProps = {
    readonly lease: Lease | null | undefined;
};

export function PropertyTenantCard({
                                       lease,
                                   }: PropertyTenantCardProps): JSX.Element | null {
    if (!lease) {
        return null;
    }

    const tenant = lease.tenantContact;
    const leaseHref = ROUTES.lease(lease.id);
    const leaseEditHref = `${ROUTES.leaseEdit(lease.id)}?${RETURN_TO_PARAM}=${encodeURIComponent(ROUTES.property(lease.propertyId))}`;

    return (
        <PropertyDetailSection>
            <SectionHeader title="Арендатор" href={leaseHref}/>

            {tenant ? (
                <div className={styles.card}>
                    <div className={styles.profile}>
            <span className={styles.avatar}>
              <Icon size="m">
                <UserSmall/>
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
                    <p className={styles.emptyText}>Арендатор не указан</p>
                    <NextLink href={leaseEditHref} className={styles.emptyLink}>
                        Указать арендатора в аренде
                    </NextLink>
                </div>
            )}
        </PropertyDetailSection>
    );
}
