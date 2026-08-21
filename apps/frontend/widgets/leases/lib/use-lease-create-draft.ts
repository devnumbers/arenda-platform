'use client';

import type { Dispatch, SetStateAction } from 'react';
import { useDraftStore } from '@/shared/lib/hooks/useDraftStore';

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
  const { draft, setDraft } = useDraftStore<LeaseCreateDraft>({
    storageKey: STORAGE_KEY,
    createDefault: () => DEFAULT_DRAFT,
    validate: validateDraft,
    isTerminal: (draft) => draft.step === 3,
  });
  return { draft, setDraft };
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
