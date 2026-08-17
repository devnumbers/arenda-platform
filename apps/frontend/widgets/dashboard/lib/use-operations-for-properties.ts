'use client';

import { useQueries } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { operationKeys } from '@/shared/api/query-keys';
import type { Property } from '@/entities/property/model/types';
import type { components } from '@/shared/api/dto';

type OperationsResponse = components['schemas']['OperationsResponse'];

const PROPERTY_OPERATIONS_PAGE_LIMIT = 100;

export type OperationsForPropertiesResult = {
  readonly operationsList: ReadonlyArray<OperationsResponse>;
  readonly isLoading: boolean;
};

async function fetchAllPropertyOperations(propertyId: string): Promise<OperationsResponse> {
  const items: OperationsResponse['items'] = [];
  let offset = 0;

  for (;;) {
    const params = new URLSearchParams({
      limit: String(PROPERTY_OPERATIONS_PAGE_LIMIT),
      offset: String(offset),
    });
    const page = await apiClient<OperationsResponse>(
      `/properties/${propertyId}/operations?${params.toString()}`,
    );
    items.push(...page.items);

    if (!page.has_more || page.next_offset == null) {
      break;
    }
    offset = page.next_offset;
  }

  return {
    items,
    limit: PROPERTY_OPERATIONS_PAGE_LIMIT,
    offset: 0,
    has_more: false,
    next_offset: null,
  };
}

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
    .filter((data): data is OperationsResponse => data !== undefined);
  const isLoading = queries.some((query) => query.isLoading);

  return { operationsList, isLoading };
}
