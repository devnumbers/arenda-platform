'use client';

import { useDraftStore } from '@/shared/lib/hooks/useDraftStore';

export type LeaseEditDraft = {
  tenantContactId: string;
  startDate: string;
  endDate: string;
  rentAmount: string;
  depositAmount: string;
  paymentDay: string;
  comment: string;
};

function getStorageKey(leaseId: string): string {
  return `lease-edit-draft-${leaseId}`;
}

export function useLeaseEditDraft(leaseId: string): {
  draft: LeaseEditDraft | undefined;
  isLoaded: boolean;
  saveDraft: (data: LeaseEditDraft) => void;
  clearDraft: () => void;
} {
  const { draft, isLoaded, saveDraft, clearDraft } = useDraftStore<LeaseEditDraft | undefined>({
    storageKey: getStorageKey(leaseId),
    createDefault: () => undefined,
    validate: validateDraft,
    persist: 'manual',
  });
  return { draft, isLoaded, saveDraft, clearDraft };
}

function validateDraft(parsed: unknown): LeaseEditDraft | undefined {
  if (parsed === null || typeof parsed !== 'object') return undefined;

  const record = parsed as Record<string, unknown>;

  if (typeof record.tenantContactId !== 'string') return undefined;
  if (typeof record.startDate !== 'string') return undefined;
  if (typeof record.endDate !== 'string') return undefined;
  if (typeof record.rentAmount !== 'string') return undefined;
  if (typeof record.depositAmount !== 'string') return undefined;
  if (typeof record.paymentDay !== 'string') return undefined;
  if (typeof record.comment !== 'string') return undefined;

  return {
    tenantContactId: record.tenantContactId,
    startDate: record.startDate,
    endDate: record.endDate,
    rentAmount: record.rentAmount,
    depositAmount: record.depositAmount,
    paymentDay: record.paymentDay,
    comment: record.comment,
  };
}
