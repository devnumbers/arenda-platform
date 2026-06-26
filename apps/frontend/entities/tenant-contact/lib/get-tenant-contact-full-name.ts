import type { TenantContact } from '@/entities/tenant-contact/model/types';

export function getTenantContactFullName(tenant: TenantContact): string {
  return [tenant.surname, tenant.name, tenant.patronymic]
    .filter(Boolean)
    .join(' ');
}
