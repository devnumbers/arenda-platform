'use client';

import { useCallback, useEffect, useState } from 'react';

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
  const [draft, setDraft] = useState<LeaseEditDraft | undefined>(undefined);
  const [isLoaded, setIsLoaded] = useState(false);

  useEffect(() => {
    // Load persisted draft after hydration; reading sessionStorage during
    // render would cause an SSR/hydration mismatch in the App Router.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setDraft(loadDraft(leaseId));
    setIsLoaded(true);
  }, [leaseId]);

  // Write to storage without touching state: the draft state is only read
  // once at form init, so a setState here would force an extra render on
  // every keystroke.
  const saveDraft = useCallback((data: LeaseEditDraft) => {
    try {
      sessionStorage.setItem(getStorageKey(leaseId), JSON.stringify(data));
    } catch {
      // Ignore storage quota / privacy mode errors.
    }
  }, [leaseId]);

  const clearDraft = useCallback(() => {
    setDraft(undefined);
    try {
      sessionStorage.removeItem(getStorageKey(leaseId));
    } catch {
      // Ignore storage quota / privacy mode errors.
    }
  }, [leaseId]);

  return { draft, isLoaded, saveDraft, clearDraft };
}

function loadDraft(leaseId: string): LeaseEditDraft | undefined {
  if (typeof window === 'undefined') return undefined;

  try {
    const raw = sessionStorage.getItem(getStorageKey(leaseId));
    if (!raw) return undefined;
    const parsed = JSON.parse(raw) as unknown;
    return validateDraft(parsed);
  } catch {
    return undefined;
  }
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
