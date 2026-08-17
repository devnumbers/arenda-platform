'use client';

import type {JSX} from 'react';
import {Skeleton} from '@heroui/react/skeleton';
import {Icon} from '@/shared/ui/icon';
import {Arendator, Arendators} from '@/shared/assets/icons';
import type {TenantContact} from '@/entities/tenant-contact/model/types';
import {SectionHeader} from '@/shared/ui/section-header';
import {EntityCard} from './EntityCard';
import {IconActionCard} from './IconActionCard';
import styles from './TenantsSection.module.css';

type TenantsSectionProps = {
    readonly tenants: TenantContact[] | undefined;
    readonly isLoading: boolean;
};

export function TenantsSection({tenants, isLoading}: TenantsSectionProps): JSX.Element | null {
    const count = tenants?.length ?? 0;

    if (isLoading) {
        return (
            <section className={styles.section}>
                <Skeleton className={styles.titleSkeleton}/>
                <div className={styles.scroll}>
                    <Skeleton className={styles.itemSkeleton}/>
                    <Skeleton className={styles.itemSkeleton}/>
                    <Skeleton className={styles.itemSkeleton}/>
                </div>
            </section>
        );
    }

    if (count === 0) {
        return null;
    }

    return (
        <section className={styles.section}>
            <SectionHeader title="Арендаторы" count={count} href="/tenants"/>
            <div className={styles.scroll}>
                {(tenants ?? []).map((tenant) => (
                    <EntityCard
                        key={tenant.id}
                        icon={
                            <Icon size="l">
                                <Arendator/>
                            </Icon>
                        }
                        href={`/tenants/${tenant.id}`}
                        title={tenant.name}
                        subtitle={tenant.surname ?? undefined}
                    />
                ))}
                <IconActionCard
                    href="/tenants"
                    icon={
                        <Icon size="l">
                            <Arendators/>
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
