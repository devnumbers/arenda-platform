export const recurringOperationKeys = {
  lists: () => ['recurring-operations', 'list'] as const,
  byProperty: (propertyId: string) =>
    ['properties', propertyId, 'recurring-operations'] as const,
  detail: (id: string) => ['recurring-operations', 'detail', id] as const,
  recurringOperations: () => recurringOperationKeys.lists(),
};
