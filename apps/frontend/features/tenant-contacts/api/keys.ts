export const tenantContactKeys = {
  all: ['tenant-contacts'] as const,
  detail: (id: string) => [...tenantContactKeys.all, id] as const,
};
