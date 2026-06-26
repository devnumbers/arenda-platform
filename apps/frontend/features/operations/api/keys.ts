export const operationKeys = {
  byProperty: (propertyId: string) =>
    ['properties', propertyId, 'operations'] as const,
  detail: (id: string) => ['operations', id] as const,
  reminders: (propertyId: string, operationId: string) =>
    [
      'properties',
      propertyId,
      'operations',
      operationId,
      'reminders',
    ] as const,
  summary: (propertyId: string) =>
    ['properties', propertyId, 'operations', 'summary'] as const,
  recurringByProperty: (propertyId: string) =>
    ['properties', propertyId, 'recurring-operations'] as const,
  recurringReminders: (propertyId: string, recurringOperationId: string) =>
    [
      'properties',
      propertyId,
      'recurring-operations',
      recurringOperationId,
      'reminders',
    ] as const,
};
