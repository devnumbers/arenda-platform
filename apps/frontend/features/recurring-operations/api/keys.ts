export const recurringOperationKeys = {
  byProperty: (propertyId: string) =>
    ['properties', propertyId, 'recurring-operations'] as const,
  detail: (id: string) => ['recurring-operations', id] as const,
  reminders: (propertyId: string, recurringOperationId: string) =>
    [
      ...recurringOperationKeys.detail(recurringOperationId),
      'reminders',
      propertyId,
    ] as const,
};
