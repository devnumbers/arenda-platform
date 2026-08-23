'use client';

import {type JSX, useMemo} from 'react';
import {useMe} from '@/features/auth';
import {useProperties} from '@/features/properties';
import {useLeases} from '@/features/leases';
import {useTenantContacts} from '@/features/tenant-contacts';
import type {OperationsFilters} from '@/features/operations';
import {formatDateForApi} from '@/entities/operation';
import {ROUTES} from '@/shared/config/routes';
import {useOperationsForProperties} from '../lib/use-operations-for-properties';
import {UserHeader} from './UserHeader';
import {NearestLease} from './NearestLease';
import {PropertiesSection} from './PropertiesSection';
import {FinanceSection} from './FinanceSection';
import {OperationsSection} from './OperationsSection';
import {TenantsSection} from './TenantsSection';
import styles from './DashboardPage.module.css';

export function DashboardPage(): JSX.Element {
    const {data: me} = useMe();
    const {data: properties, isLoading: propertiesLoading} = useProperties();
    const {data: leases, isLoading: leasesLoading} = useLeases();
    const {data: tenantContacts, isLoading: tenantContactsLoading} = useTenantContacts();
    const {isLoading: operationsLoading} = useOperationsForProperties(properties);

    const upcomingFilters = useMemo<OperationsFilters>(
        () => ({
            status: ['pending'],
            from: formatDateForApi(new Date()),
            sort: 'operation_date_asc',
            limit: 3,
            exclude_archived_properties: true,
        }),
        [],
    );

    const overdueFilters = useMemo<OperationsFilters>(
        () => ({
            status: ['overdue'],
            sort: 'operation_date_asc',
            limit: 3,
            exclude_archived_properties: true,
        }),
        [],
    );

    return (
        <div className={styles.page}>
            <UserHeader name={me?.name} tariff={me?.subscription?.tariff.name}/>
            <NearestLease leases={leases} properties={properties} isLoading={leasesLoading || propertiesLoading}/>
            <PropertiesSection properties={properties} isLoading={propertiesLoading}/>
            <FinanceSection properties={properties} isLoading={propertiesLoading || operationsLoading}/>
            <OperationsSection
                title="Ближайшие операции"
                href={`${ROUTES.financeOperations}?status=pending&period=all`}
                filters={upcomingFilters}
            />
            <OperationsSection
                title="Просроченные операции"
                href={`${ROUTES.financeOperations}?status=overdue&period=all`}
                filters={overdueFilters}
            />
            <TenantsSection tenants={tenantContacts} isLoading={tenantContactsLoading}/>
        </div>
    );
}
