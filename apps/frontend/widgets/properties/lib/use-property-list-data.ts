'use client';

import { useMemo } from 'react';
import { useProperties } from '@/features/properties/api';
import { useLeases } from '@/features/leases/api';
import type { components } from '@/shared/api/generated';

type PropertyResponse = components['schemas']['PropertyResponse'];
type LeaseResponse = components['schemas']['LeaseResponse'];

export type PropertyWithLease = PropertyResponse & {
  activeLease?: LeaseResponse;
};

export function usePropertyListData() {
  const propertiesQuery = useProperties();
  const leasesQuery = useLeases();

  const data = useMemo<PropertyWithLease[] | undefined>(() => {
    if (!propertiesQuery.data) return undefined;
    const leaseByProperty = new Map<string, LeaseResponse>();

    leasesQuery.data?.forEach((lease) => {
      if (!leaseByProperty.has(lease.property_id)) {
        leaseByProperty.set(lease.property_id, lease);
      }
    });

    return propertiesQuery.data.map((property) => ({
      ...property,
      activeLease: leaseByProperty.get(property.id),
    }));
  }, [propertiesQuery.data, leasesQuery.data]);

  return {
    data,
    isLoading: propertiesQuery.isLoading || leasesQuery.isLoading,
    isError: propertiesQuery.isError || leasesQuery.isError,
    error: propertiesQuery.error ?? leasesQuery.error,
  };
}
