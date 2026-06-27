export const leaseKeys = {
  all: ['leases'] as const,
  detail: (id: string) => [...leaseKeys.all, id] as const,
  byProperty: (propertyId: string) =>
    [...leaseKeys.all, 'property', propertyId] as const,
  reminders: (id: string) => [...leaseKeys.all, id, 'reminders'] as const,
};
