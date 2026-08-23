'use client';

import {useMemo, type JSX} from 'react';
import NextLink from 'next/link';
import {useTenantContacts} from '@/features/tenant-contacts';
import {PageHeader} from '@/shared/ui/page-header';
import {Icon} from '@/shared/ui/icon';
import {ArendatorAdd} from '@/shared/assets/icons';
import {ROUTES} from '@/shared/config/routes';
import {TenantSection} from './TenantSection';
import {TenantsLoading} from './TenantsLoading';
import {TenantsError} from './TenantsError';
import {TenantsEmptyState} from './TenantsEmptyState';
import styles from './TenantsPage.module.css';

export function TenantsPage(): JSX.Element {
    const query = useTenantContacts();

    const {active, past} = useMemo(() => {
        const tenants = query.data ?? [];
        const activeContacts = tenants.filter((t) => t.isActive);
        const pastContacts = tenants.filter((t) => !t.isActive);
        return {active: activeContacts, past: pastContacts};
    }, [query.data]);

    return (
        <div className={styles.root}>
            <PageHeader
                title="Арендаторы"
                actions={
                    <NextLink
                        href={ROUTES.tenantNew}
                        className={styles.addButton}
                        aria-label="Добавить арендатора"
                    >
                        <Icon size="l">
                            <ArendatorAdd/>
                        </Icon>
                    </NextLink>
                }
            />

            {query.isPending && <TenantsLoading/>}

            {!query.isPending && query.isError && (
                <TenantsError onRetry={() => void query.refetch()} isLoading={query.isFetching}/>
            )}

            {!query.isPending && !query.isError && query.data.length === 0 && (
                <TenantsEmptyState/>
            )}

            {!query.isPending && !query.isError && query.data.length > 0 && (
                <>
                    <TenantSection title="Текущие арендаторы" tenants={active}/>
                    <TenantSection title="Остальные" tenants={past}/>
                </>
            )}
        </div>
    );
}
