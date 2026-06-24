'use client';

import type { JSX } from 'react';
import { useMe } from '@/features/auth/api/hooks';
import { useProperties } from '@/features/properties/api/hooks';
import { useLeases } from '@/features/leases/api/hooks';
import { useReminders } from '@/features/reminders/api/hooks';
import { useOperationsForProperties } from '../lib/use-operations-for-properties';
import { UserHeader } from './UserHeader';
import { NearestLease } from './NearestLease';
import { PropertiesSection } from './PropertiesSection';
import { FinanceSection } from './FinanceSection';
import { PaymentsSection } from './PaymentsSection';
import { TenantsSection } from './TenantsSection';
import styles from './DashboardPage.module.css';

export function DashboardPage(): JSX.Element {
  const { data: me } = useMe();
  const { data: properties, isLoading: propertiesLoading } = useProperties();
  const { data: leases, isLoading: leasesLoading } = useLeases();
  const { data: reminders, isLoading: remindersLoading } = useReminders(10, 0);
  const { isLoading: operationsLoading } = useOperationsForProperties(properties);

  return (
    <div className={styles.page}>
      <UserHeader name={me?.name} tariff={me?.subscription?.tariff?.name} />
      <NearestLease leases={leases} properties={properties} isLoading={leasesLoading || propertiesLoading} />
      <PropertiesSection properties={properties} isLoading={propertiesLoading} />
      <FinanceSection properties={properties} isLoading={propertiesLoading || operationsLoading} />
      <PaymentsSection reminders={reminders?.items} properties={properties} isLoading={remindersLoading} />
      <TenantsSection leases={leases} isLoading={leasesLoading} />
    </div>
  );
}
