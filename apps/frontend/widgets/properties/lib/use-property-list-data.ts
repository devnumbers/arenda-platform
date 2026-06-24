'use client';

import { useMemo } from 'react';
import { useProperties } from '@/features/properties/api/hooks';
import { useLeases } from '@/features/leases/api/hooks';
import { ApiError } from '@/shared/api/errors';
import type { components } from '@/shared/api/generated';

type PropertyResponse = components['schemas']['PropertyResponse'];
type LeaseResponse = components['schemas']['LeaseResponse'];

export type PropertyWithLease = PropertyResponse & {
  activeLease?: LeaseResponse;
};

type UsePropertyListDataReturn = {
  data: PropertyWithLease[] | undefined;
  isLoading: boolean;
  isError: boolean;
  error: {
    propertiesError: ApiError | null;
    leasesError: ApiError | null;
  };
};

function selectActiveLease(leases: LeaseResponse[], propertyId: string): LeaseResponse | undefined {
  let activeLease: LeaseResponse | undefined;

  for (const lease of leases) {
    if (lease.property_id !== propertyId || lease.status !== 'active') continue;

    if (!activeLease) {
      activeLease = lease;
      continue;
    }

    const leaseStart = new Date(lease.start_date).getTime();
    const currentStart = new Date(activeLease.start_date).getTime();

    if (leaseStart > currentStart || (leaseStart === currentStart && lease.id > activeLease.id)) {
      activeLease = lease;
    }
  }

  return activeLease;
}

export function usePropertyListData(): UsePropertyListDataReturn {
  const propertiesQuery = useProperties();
  const leasesQuery = useLeases();

  const data = useMemo<PropertyWithLease[] | undefined>(() => {
    if (!propertiesQuery.data) return undefined;

    return propertiesQuery.data.map((property) => ({
      ...property,
      activeLease: leasesQuery.data
        ? selectActiveLease(leasesQuery.data, property.id)
        : undefined,
    }));
  }, [propertiesQuery.data, leasesQuery.data]);

  return {
    data,
    isLoading: propertiesQuery.isLoading || leasesQuery.isLoading,
    isError: propertiesQuery.isError || leasesQuery.isError,
    error: {
      propertiesError: propertiesQuery.error,
      leasesError: leasesQuery.error,
    },
  };
}
