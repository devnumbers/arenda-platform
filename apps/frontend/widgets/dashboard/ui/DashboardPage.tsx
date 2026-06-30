'use client';

import type { JSX } from 'react';
import { useMe } from '@/features/auth/api/hooks';
import { useProperties } from '@/features/properties/api/hooks';
import { useLeases } from '@/features/leases/api/hooks';
import { useOperationsForProperties } from '../lib/use-operations-for-properties';
import { UserHeader } from './UserHeader';
import { NearestLease } from './NearestLease';
import { PropertiesSection } from './PropertiesSection';
import { FinanceSection } from './FinanceSection';
import { UpcomingOperationsSection } from './UpcomingOperationsSection';
import { TenantsSection } from './TenantsSection';
import styles from './DashboardPage.module.css';

export function DashboardPage(): JSX.Element {
  const { data: me } = useMe();
  const { data: properties, isLoading: propertiesLoading } = useProperties();
  const { data: leases, isLoading: leasesLoading } = useLeases();
  const { isLoading: operationsLoading } = useOperationsForProperties(properties);

  return (
    <div className={styles.page}>
      <UserHeader name={me?.name} tariff={me?.subscription?.tariff?.name} />
      <NearestLease leases={leases} properties={properties} isLoading={leasesLoading || propertiesLoading} />
      <PropertiesSection properties={properties} isLoading={propertiesLoading} />
      <FinanceSection properties={properties} isLoading={propertiesLoading || operationsLoading} />
      <UpcomingOperationsSection />
      <TenantsSection leases={leases} isLoading={leasesLoading} />
    </div>
  );
}
