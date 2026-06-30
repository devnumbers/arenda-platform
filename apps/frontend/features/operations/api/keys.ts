export type OperationKeyFilters = {
  type?: string | string[];
  status?: string | string[];
  category?: string | string[];
  property_id?: string;
  from?: string;
  to?: string;
  recurring_operation_id?: string;
  lease_id?: string;
  sort?: string;
  limit?: string;
  offset?: string;
};

export const operationKeys = {
  lists: () => ['operations', 'list'] as const,
  infiniteLists: () => ['operations', 'infinite'] as const,
  byProperty: (
    propertyId: string,
    filters?: Omit<OperationKeyFilters, 'property_id'>,
  ) => ['properties', propertyId, 'operations', filters ?? {}] as const,
  detail: (id: string) => ['operations', 'detail', id] as const,
  operations: (filters: OperationKeyFilters) =>
    [...operationKeys.lists(), filters] as const,
  infiniteOperations: (filters: Omit<OperationKeyFilters, 'offset'>) =>
    [...operationKeys.infiniteLists(), filters] as const,
  summary: (propertyId: string) =>
    ['properties', propertyId, 'operations', 'summary'] as const,
  recurringByProperty: (propertyId: string) =>
    ['properties', propertyId, 'recurring-operations'] as const,
};
