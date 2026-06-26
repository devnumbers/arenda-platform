'use client';

import { useState, useEffect, useCallback } from 'react';
import type { Dispatch, SetStateAction } from 'react';

export type TenantCreateStep = 'form' | 'success';

export type TenantCreateDraft = {
  step: TenantCreateStep;
  name: string;
  surname: string;
  patronymic: string;
  phone: string;
  email: string;
  comment: string;
};

const STORAGE_KEY = 'tenant-create-draft';

const DEFAULT_DRAFT: TenantCreateDraft = {
  step: 'form',
  name: '',
  surname: '',
  patronymic: '',
  phone: '',
  email: '',
  comment: '',
};

export function useTenantCreateDraft(): {
  draft: TenantCreateDraft;
  setDraft: Dispatch<SetStateAction<TenantCreateDraft>>;
  clearDraft: () => void;
} {
  const [draft, setDraft] = useState<TenantCreateDraft>(DEFAULT_DRAFT);

  useEffect(() => {
    // Load persisted draft after hydration to avoid SSR/hydration mismatch.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setDraft(loadDraft());
  }, []);

  useEffect(() => {
    try {
      // Persist both form and success states so a refresh returns to the
      // same step (e.g. success screen survives reload). The draft is cleared
      // explicitly on close or success-button navigation.
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

  return { draft, setDraft, clearDraft };
}

function loadDraft(): TenantCreateDraft {
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

function isStep(value: unknown): value is TenantCreateStep {
  return value === 'form' || value === 'success';
}

function validateDraft(parsed: unknown): TenantCreateDraft {
  if (parsed === null || typeof parsed !== 'object') return DEFAULT_DRAFT;

  const record = parsed as Record<string, unknown>;

  if (!isStep(record.step)) return DEFAULT_DRAFT;
  if (!isString(record.name)) return DEFAULT_DRAFT;
  if (!isString(record.surname)) return DEFAULT_DRAFT;
  if (!isString(record.patronymic)) return DEFAULT_DRAFT;
  if (!isString(record.phone)) return DEFAULT_DRAFT;
  if (!isString(record.email)) return DEFAULT_DRAFT;
  if (!isString(record.comment)) return DEFAULT_DRAFT;

  return {
    step: record.step,
    name: record.name,
    surname: record.surname,
    patronymic: record.patronymic,
    phone: record.phone,
    email: record.email,
    comment: record.comment,
  };
}
