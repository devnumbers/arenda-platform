export const leaseKeys = {
  all: ['leases'] as const,
  detail: (id: string) => [...leaseKeys.all, id] as const,
  reminders: (id: string) => [...leaseKeys.all, id, 'reminders'] as const,
};
