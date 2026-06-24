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

function compareActiveLeases(a: LeaseResponse, b: LeaseResponse): number {
  // Pick the "more current" active lease:
  // - a later start_date wins;
  // - leases with an invalid/unparseable start_date are treated as older
  //   than any lease with a valid date, so valid dates always win;
  // - if start_date is identical, the lease with the greater id wins
  //   (lexicographic comparison of UUIDs is sufficient here).
  const aStart = new Date(a.start_date).getTime();
  const bStart = new Date(b.start_date).getTime();
  const aValid = !Number.isNaN(aStart);
  const bValid = !Number.isNaN(bStart);

  if (aValid && !bValid) return 1;
  if (!aValid && bValid) return -1;

  if (!aValid && !bValid) {
    if (a.id > b.id) return 1;
    if (a.id < b.id) return -1;
    return 0;
  }

  if (aStart !== bStart) return aStart - bStart;

  if (a.id > b.id) return 1;
  if (a.id < b.id) return -1;
  return 0;
}

export function usePropertyListData(): UsePropertyListDataReturn {
  const propertiesQuery = useProperties();
  const leasesQuery = useLeases();

  const data = useMemo<PropertyWithLease[] | undefined>(() => {
    if (!propertiesQuery.data) return undefined;

    // Index active leases by property_id in a single O(L) pass, keeping
    // the best candidate per property. The final join over properties is O(P).
    const activeLeaseByProperty = new Map<string, LeaseResponse>();

    for (const lease of leasesQuery.data ?? []) {
      if (lease.status !== 'active') continue;

      const current = activeLeaseByProperty.get(lease.property_id);

      if (!current || compareActiveLeases(lease, current) > 0) {
        activeLeaseByProperty.set(lease.property_id, lease);
      }
    }

    return propertiesQuery.data.map((property) => ({
      ...property,
      activeLease: activeLeaseByProperty.get(property.id),
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
