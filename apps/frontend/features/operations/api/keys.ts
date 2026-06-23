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
};
