'use client';

import type { Dispatch, SetStateAction } from 'react';
import { useDraftStore } from '@/shared/lib/hooks/useDraftStore';
import { isPhoneValid } from '@/shared/lib/phone';

export type LoginStep = 'phone' | 'email' | 'code';

export type LoginDraft = {
  step: LoginStep;
  phone: string;
  email: string;
};

const STORAGE_KEY = 'login-draft';

const DEFAULT_DRAFT: LoginDraft = {
  step: 'phone',
  phone: '',
  email: '',
};

export function useLoginDraft(): {
  draft: LoginDraft;
  setDraft: Dispatch<SetStateAction<LoginDraft>>;
  clearDraft: () => void;
} {
  const { draft, setDraft, clearDraft } = useDraftStore<LoginDraft>({
    storageKey: STORAGE_KEY,
    createDefault: () => DEFAULT_DRAFT,
    validate: validateDraft,
  });
  return { draft, setDraft, clearDraft };
}

function isString(value: unknown): value is string {
  return typeof value === 'string';
}

function isStep(value: unknown): value is LoginStep {
  return value === 'phone' || value === 'email' || value === 'code';
}

function validateDraft(parsed: unknown): LoginDraft {
  if (parsed === null || typeof parsed !== 'object') return DEFAULT_DRAFT;

  const record = parsed as Record<string, unknown>;

  if (!isStep(record.step)) return DEFAULT_DRAFT;
  if (!isString(record.phone)) return DEFAULT_DRAFT;
  if (!isString(record.email)) return DEFAULT_DRAFT;

  // Steps beyond "phone" require a valid phone; otherwise restart the flow.
  const step: LoginStep = record.step !== 'phone' && !isPhoneValid(record.phone) ? 'phone' : record.step;

  return {
    step,
    phone: record.phone,
    email: record.email,
  };
}
