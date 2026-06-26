import { formatAgo } from '@/shared/lib/format-duration';
import { getLeaseMonthCount } from '@/shared/lib/format-lease-month';
import type { TenantContact } from '@/entities/tenant-contact/model/types';

export function getTenantSubtitle(contact: TenantContact): string | undefined {
  if (contact.activeLease) {
    const startDate = new Date(contact.activeLease.startDate);
    if (Number.isNaN(startDate.getTime())) return undefined;

    const n = getLeaseMonthCount(contact.activeLease.startDate) + 1;
    return `${n}-й месяц аренды`;
  }

  if (contact.lastLease?.endDate) {
    const endDate = new Date(contact.lastLease.endDate);
    if (Number.isNaN(endDate.getTime())) return undefined;

    return `Аренда завершена ${formatAgo(contact.lastLease.endDate)}`;
  }

  return undefined;
}
