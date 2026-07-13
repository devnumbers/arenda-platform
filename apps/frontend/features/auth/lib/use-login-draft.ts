'use client';

import { useState, useEffect, useCallback } from 'react';
import type { Dispatch, SetStateAction } from 'react';
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
  isLoaded: boolean;
} {
  const [draft, setDraft] = useState<LoginDraft>(DEFAULT_DRAFT);
  const [isLoaded, setIsLoaded] = useState(false);

  useEffect(() => {
    // Load persisted draft after hydration to avoid SSR/hydration mismatch.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setDraft(loadDraft());
    setIsLoaded(true);
  }, []);

  useEffect(() => {
    try {
      sessionStorage.setItem(STORAGE_KEY, JSON.stringify(draft));
    } catch {
      // Ignore storage quota / privacy mode errors.
    }
  }, [draft]);

  const clearDraft = useCallback(() => {
    setDraft(DEFAULT_DRAFT);
    try {
      sessionStorage.removeItem(STORAGE_KEY);
    } catch {
      // Ignore storage errors.
    }
  }, []);

  return { draft, setDraft, clearDraft, isLoaded };
}

function loadDraft(): LoginDraft {
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
