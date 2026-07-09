'use client';

import { useMemo, type JSX } from 'react';
import { useTenantContact } from '@/features/tenant-contacts/api';
import { useLeases } from '@/features/leases/api';
import { mapLeaseResponse } from '@/entities/lease/model/mappers';
import type { Lease } from '@/entities/lease/model/types';
import { TenantDetailHeader } from './TenantDetailHeader';
import { TenantInfoSection } from './TenantInfoSection';
import { TenantLeaseSection } from './TenantLeaseSection';
import { TenantCommentSection } from './TenantCommentSection';
import { TenantDetailLoading } from './TenantDetailLoading';
import { TenantDetailError } from './TenantDetailError';
import { TenantActionMenu } from './TenantActionMenu';
import { getTenantContactFullName } from '@/entities/tenant-contact/lib/get-tenant-contact-full-name';
import styles from './TenantDetailPage.module.css';

function findCurrentLeaseByTenant(
  leases: ReadonlyArray<Lease>,
  tenantId: string,
): Lease | undefined {
  return leases.find(
    (lease) =>
      lease.tenantContactId === tenantId &&
      lease.status !== 'completed' &&
      lease.status !== 'archived',
  );
}

export type TenantDetailPageProps = {
  readonly id: string;
};

export function TenantDetailPage({ id }: TenantDetailPageProps): JSX.Element {
  const tenantQuery = useTenantContact(id);
  const leasesQuery = useLeases();

  const currentLease = useMemo(() => {
    const leases = (leasesQuery.data ?? []).map(mapLeaseResponse);
    return findCurrentLeaseByTenant(leases, id);
  }, [leasesQuery.data, id]);

  const isLoading = tenantQuery.isPending || leasesQuery.isPending;
  const isError =
    tenantQuery.isError || leasesQuery.isError || !tenantQuery.data;
  const canEdit = tenantQuery.data !== undefined && !isLoading;

  const tenant = tenantQuery.data;
  const title = tenant
    ? getTenantContactFullName(tenant) || tenant.name
    : 'Арендатор';

  return (
    <main className={styles.root}>
      <TenantDetailHeader
        title={title}
        actions={<TenantActionMenu tenantId={id} canEdit={canEdit} />}
      />

      {isLoading && <TenantDetailLoading />}

      {!isLoading && isError && (
        <TenantDetailError
          onRetry={() => {
            tenantQuery.refetch();
            leasesQuery.refetch();
          }}
          isLoading={tenantQuery.isFetching || leasesQuery.isFetching}
        />
      )}

      {!isLoading && !isError && tenant && (
        <>
          <TenantInfoSection tenant={tenant} />
          {currentLease && <TenantLeaseSection lease={currentLease} />}
          {tenant.comment && <TenantCommentSection comment={tenant.comment} />}
        </>
      )}
    </main>
  );
}
