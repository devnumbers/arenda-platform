export const recurringOperationKeys = {
  byProperty: (propertyId: string) =>
    ['properties', propertyId, 'recurring-operations'] as const,
  detail: (id: string) => ['recurring-operations', id] as const,
  recurringOperations: () => ['recurring-operations'] as const,
};
