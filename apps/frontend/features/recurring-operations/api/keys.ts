export const recurringOperationKeys = {
  byProperty: (propertyId: string) =>
    ['properties', propertyId, 'recurring-operations'] as const,
  detail: (id: string) => ['recurring-operations', id] as const,
  recurringOperations: () => ['recurring-operations'] as const,
  reminders: (propertyId: string, recurringOperationId: string) =>
    [
      'properties',
      propertyId,
      'recurring-operations',
      recurringOperationId,
      'reminders',
    ] as const,
};
