'use client';

import { useMemo } from 'react';
import {useArchivedProperties, useProperties} from '@/features/properties/api/hooks';
import { useLeases } from '@/features/leases/api/hooks';
import type {PropertiesViewMode} from './apply-filters';
import { ApiError } from '@/shared/api/errors';
import { mapLeaseResponse } from '@/entities/lease/model/mappers';
import type { Property } from '@/entities/property/model/types';
import type { Lease } from '@/entities/lease/model/types';
import { getEffectiveLeaseStatus, isOpenLease } from '@/entities/lease/lib/status';

export type PropertyWithLease = Property;

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
  const statusPriority: Record<Lease['status'], number> = {
    requires_action: 3,
    active: 2,
    awaiting_start: 1,
    completed: 0,
    archived: 0,
  };
  const aStatus = getEffectiveLeaseStatus(a);
  const bStatus = getEffectiveLeaseStatus(b);

  if (statusPriority[aStatus] !== statusPriority[bStatus]) {
    return statusPriority[aStatus] - statusPriority[bStatus];
  }

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

export function usePropertyListData(mode: PropertiesViewMode): UsePropertyListDataReturn {
  const propertiesQuery = useProperties({enabled: mode === 'active'});
  const archivedPropertiesQuery = useArchivedProperties({enabled: mode === 'archived'});
  const leasesQuery = useLeases();

  const sourceQuery = mode === 'archived' ? archivedPropertiesQuery : propertiesQuery;

  const data = useMemo<PropertyWithLease[] | undefined>(() => {
    if (!sourceQuery.data) return undefined;

    const openLeaseByProperty = new Map<string, Lease>();

    for (const lease of leasesQuery.data ?? []) {
      const mappedLease = mapLeaseResponse(lease);
      if (!isOpenLease(mappedLease)) {
        continue;
      }

      const effectiveLease = {
        ...mappedLease,
        status: getEffectiveLeaseStatus(mappedLease),
      };
      const current = openLeaseByProperty.get(effectiveLease.propertyId);

      if (!current || compareLeaseRelevance(effectiveLease, current) > 0) {
        openLeaseByProperty.set(effectiveLease.propertyId, effectiveLease);
      }
    }

    return sourceQuery.data.map((property) => {
      const activeLease = openLeaseByProperty.get(property.id) ?? null;
      return {
        ...property,
        activeLease,
      };
    });
  }, [sourceQuery.data, leasesQuery.data]);

  return {
    data,
    isLoading: sourceQuery.isLoading || leasesQuery.isLoading,
    isFetching: sourceQuery.isFetching || leasesQuery.isFetching,
    isError: sourceQuery.isError || leasesQuery.isError,
    error: {
      propertiesError: sourceQuery.error,
      leasesError: leasesQuery.error,
    },
    refetch: () => {
      void sourceQuery.refetch();
      void leasesQuery.refetch();
    },
  };
}
