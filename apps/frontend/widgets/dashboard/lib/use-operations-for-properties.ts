'use client';

import { useQueries } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { operationKeys } from '@/features/operations/api/keys';
import type { Property } from '@/entities/property/model/types';
import type { components } from '@/shared/api/generated';

type OperationsResponse = components['schemas']['OperationsResponse'];

export type OperationsForPropertiesResult = {
  readonly operationsList: ReadonlyArray<OperationsResponse>;
  readonly isLoading: boolean;
};

export function useOperationsForProperties(
  properties: Property[] | undefined,
): OperationsForPropertiesResult {
  const propertyIds = properties?.map((property) => property.id) ?? [];
  const queries = useQueries({
    queries: propertyIds.map((propertyId) => ({
      queryKey: operationKeys.byProperty(propertyId),
      queryFn: () =>
        apiClient<OperationsResponse>(`/properties/${propertyId}/operations`),
      enabled: Boolean(propertyId),
    })),
  });

  const operationsList = queries
    .map((query) => query.data)
    .filter((data): data is OperationsResponse => data !== undefined);
  const isLoading = queries.some((query) => query.isLoading);

  return { operationsList, isLoading };
}
