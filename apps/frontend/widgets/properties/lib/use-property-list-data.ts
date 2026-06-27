'use client';

import { useMemo } from 'react';
import { useProperties } from '@/features/properties/api/hooks';
import { useLeases } from '@/features/leases/api/hooks';
import { ApiError } from '@/shared/api/errors';
import { mapLeaseResponse } from '@/entities/lease/model/mappers';
import type { Property } from '@/entities/property/model/types';
import type { Lease } from '@/entities/lease/model/types';

export type PropertyWithLease = Property & {
  activeLease?: Lease;
  lastLease?: Lease;
};

type UsePropertyListDataReturn = {
  data: PropertyWithLease[] | undefined;
  isLoading: boolean;
  isFetching: boolean;
  isError: boolean;
  error: {
    propertiesError: ApiError | null;
    leasesError: ApiError | null;
  };
  refetch: () => void;
};

function compareLeaseRelevance(a: Lease, b: Lease): number {
  // Active leases always beat completed ones. Among the same status,
  // the later start_date wins. Invalid/unparseable start dates are
  // treated as older than valid dates.
  if (a.status === 'active' && b.status !== 'active') return 1;
  if (a.status !== 'active' && b.status === 'active') return -1;

  const aStart = a.startDate ? new Date(a.startDate).getTime() : NaN;
  const bStart = b.startDate ? new Date(b.startDate).getTime() : NaN;
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

    // Index the best lease candidate per property in a single O(L) pass.
    // The candidate is either the current active lease or, if none exists,
    // the most recently started completed lease. The final join is O(P).
    const bestLeaseByProperty = new Map<string, Lease>();

    for (const lease of leasesQuery.data ?? []) {
      const mappedLease = mapLeaseResponse(lease);
      if (mappedLease.status !== 'active' && mappedLease.status !== 'completed') {
        continue;
      }

      const current = bestLeaseByProperty.get(mappedLease.propertyId);

      if (!current || compareLeaseRelevance(mappedLease, current) > 0) {
        bestLeaseByProperty.set(mappedLease.propertyId, mappedLease);
      }
    }

    return propertiesQuery.data.map((property) => {
      const bestLease = bestLeaseByProperty.get(property.id);
      return {
        ...property,
        activeLease: bestLease?.status === 'active' ? bestLease : undefined,
        lastLease: bestLease,
      };
    });
  }, [propertiesQuery.data, leasesQuery.data]);

  return {
    data,
    isLoading: propertiesQuery.isLoading || leasesQuery.isLoading,
    isFetching: propertiesQuery.isFetching || leasesQuery.isFetching,
    isError: propertiesQuery.isError || leasesQuery.isError,
    error: {
      propertiesError: propertiesQuery.error,
      leasesError: leasesQuery.error,
    },
    refetch: () => {
      void propertiesQuery.refetch();
      void leasesQuery.refetch();
    },
  };
}
