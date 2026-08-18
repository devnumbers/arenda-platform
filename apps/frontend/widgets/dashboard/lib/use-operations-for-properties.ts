'use client';

import { useQueries } from '@tanstack/react-query';
import { operationKeys } from '@/shared/api/query-keys';
import { fetchAllPropertyOperations } from '@/features/operations';
import type { OperationsPage } from '@/entities/operation';
import type { Property } from '@/entities/property';

export type OperationsForPropertiesResult = {
  readonly operationsList: ReadonlyArray<OperationsPage>;
  readonly isLoading: boolean;
};

export function useOperationsForProperties(
  properties: Property[] | undefined,
): OperationsForPropertiesResult {
  const propertyIds = properties?.map((property) => property.id) ?? [];
  const queries = useQueries({
    queries: propertyIds.map((propertyId) => ({
      queryKey: operationKeys.byProperty(propertyId, { limit: 'all' }),
      queryFn: () => fetchAllPropertyOperations(propertyId),
      enabled: Boolean(propertyId),
    })),
  });

  const operationsList = queries
    .map((query) => query.data)
    .filter((data): data is OperationsPage => data !== undefined);
  const isLoading = queries.some((query) => query.isLoading);

  return { operationsList, isLoading };
}
