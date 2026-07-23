'use client';

import { useState, useEffect } from 'react';
import type { Dispatch, SetStateAction } from 'react';

export type LeaseCreateStep = 1 | 2 | 3;

export type LeaseCreateDraft = {
  step: LeaseCreateStep;
  rentAmount?: string;
  depositAmount?: string;
  paymentDay?: number;
  startDate?: string;
  endDate?: string;
  tenantContactId?: string;
};

const STORAGE_KEY = 'lease-create-draft';

const DEFAULT_DRAFT: LeaseCreateDraft = { step: 1 };

export function useLeaseCreateDraft(): {
  draft: LeaseCreateDraft;
  setDraft: Dispatch<SetStateAction<LeaseCreateDraft>>;
} {
  const [draft, setDraft] = useState<LeaseCreateDraft>(DEFAULT_DRAFT);

  useEffect(() => {
    // Load persisted draft after hydration; reading sessionStorage during
    // render would cause an SSR/hydration mismatch in the App Router.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setDraft(loadDraft());
  }, []);

  useEffect(() => {
    try {
      if (draft.step === 3) {
        sessionStorage.removeItem(STORAGE_KEY);
        return;
      }
      sessionStorage.setItem(STORAGE_KEY, JSON.stringify(draft));
    } catch {
      // Ignore storage quota / privacy mode errors.
    }
  }, [draft]);

  return { draft, setDraft };
}

function loadDraft(): LeaseCreateDraft {
  if (typeof window === 'undefined') return DEFAULT_DRAFT;

  try {
    const raw = sessionStorage.getItem(STORAGE_KEY);
    if (!raw) return DEFAULT_DRAFT;
    const parsed = JSON.parse(raw) as unknown;
    return validateDraft(parsed);
  } catch {
    return DEFAULT_DRAFT;
  }
}

function isOptionalString(value: unknown): value is string | undefined {
  return value === undefined || typeof value === 'string';
}

function isOptionalPaymentDay(value: unknown): value is number | undefined {
  return (
    value === undefined ||
    (typeof value === 'number' && Number.isInteger(value) && value >= 1 && value <= 31)
  );
}

function validateDraft(parsed: unknown): LeaseCreateDraft {
  if (parsed === null || typeof parsed !== 'object') return DEFAULT_DRAFT;

  const record = parsed as Record<string, unknown>;

  const step = Number(record.step);
  if (!Number.isInteger(step) || step < 1 || step > 3) return DEFAULT_DRAFT;

  if (!isOptionalString(record.rentAmount)) return DEFAULT_DRAFT;
  if (!isOptionalString(record.depositAmount)) return DEFAULT_DRAFT;
  if (!isOptionalPaymentDay(record.paymentDay)) return DEFAULT_DRAFT;
  if (!isOptionalString(record.startDate)) return DEFAULT_DRAFT;
  if (!isOptionalString(record.endDate)) return DEFAULT_DRAFT;
  if (!isOptionalString(record.tenantContactId)) return DEFAULT_DRAFT;

  return {
    step: step as LeaseCreateStep,
    ...(record.rentAmount !== undefined && { rentAmount: record.rentAmount }),
    ...(record.depositAmount !== undefined && { depositAmount: record.depositAmount }),
    ...(record.paymentDay !== undefined && { paymentDay: record.paymentDay }),
    ...(record.startDate !== undefined && { startDate: record.startDate }),
    ...(record.endDate !== undefined && { endDate: record.endDate }),
    ...(record.tenantContactId !== undefined && { tenantContactId: record.tenantContactId }),
  };
}
