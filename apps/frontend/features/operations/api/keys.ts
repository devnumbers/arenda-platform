export const operationKeys = {
  byProperty: (propertyId: string) =>
    ['properties', propertyId, 'operations'] as const,
  detail: (id: string) => ['operations', id] as const,
  reminders: (propertyId: string, operationId: string) =>
    [...operationKeys.detail(operationId), 'reminders', propertyId] as const,
};
