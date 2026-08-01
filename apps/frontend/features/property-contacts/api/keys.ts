export const propertyContactKeys = {
  all: ['property-contacts'] as const,
  list: (propertyId: string) =>
    [...propertyContactKeys.all, 'list', propertyId] as const,
  detail: (propertyId: string, contactId: string) =>
    [...propertyContactKeys.all, 'detail', propertyId, contactId] as const,
};
