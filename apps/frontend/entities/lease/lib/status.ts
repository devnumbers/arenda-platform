import type { LeaseStatus } from '@/entities/lease/model/types';

const OPEN_LEASE_STATUSES = new Set<LeaseStatus>([
  'awaiting_start',
  'active',
  'requires_action',
]);

function dateOnly(value: Date): Date {
  return new Date(value.getFullYear(), value.getMonth(), value.getDate());
}

function parseLocalDate(date: string): Date | null {
  const parsed = new Date(`${date}T00:00:00`);
  if (Number.isNaN(parsed.getTime())) {
    return null;
  }
  return parsed;
}

function isPastDate(date: string, now: Date = new Date()): boolean {
  const parsed = parseLocalDate(date);
  return parsed !== null && parsed < dateOnly(now);
}

function isFutureDate(date: string, now: Date = new Date()): boolean {
  const parsed = parseLocalDate(date);
  return parsed !== null && parsed > dateOnly(now);
}

export function isOpenLeaseStatus(status: LeaseStatus): boolean {
  return OPEN_LEASE_STATUSES.has(status);
}

export function getEffectiveLeaseStatus(
  lease: {
    readonly status: LeaseStatus;
    readonly startDate?: string;
    readonly endDate?: string;
  },
  now?: Date,
): LeaseStatus {
  if (!isOpenLeaseStatus(lease.status)) {
    return lease.status;
  }

  if (lease.startDate && isFutureDate(lease.startDate, now)) {
    return 'awaiting_start';
  }

  if (lease.endDate && isPastDate(lease.endDate, now)) {
    return 'requires_action';
  }

  if (lease.startDate) {
    return 'active';
  }

  return lease.status;
}

export function isOpenLease(lease: {
  readonly status: LeaseStatus;
  readonly startDate?: string;
  readonly endDate?: string;
}): boolean {
  return isOpenLeaseStatus(getEffectiveLeaseStatus(lease));
}
