'use client';

import type { Dispatch, SetStateAction } from 'react';
import { useDraftStore } from '@/shared/lib/hooks/useDraftStore';

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
  isLoaded: boolean;
  setDraft: Dispatch<SetStateAction<TenantCreateDraft>>;
  clearDraft: () => void;
} {
  const { draft, isLoaded, setDraft, clearDraft } = useDraftStore<TenantCreateDraft>({
    storageKey: STORAGE_KEY,
    // Persist both form and success states so a refresh returns to the
    // same step (e.g. success screen survives reload). The draft is cleared
    // explicitly on close or success-button navigation.
    createDefault: () => DEFAULT_DRAFT,
    validate: validateDraft,
  });
  return { draft, isLoaded, setDraft, clearDraft };
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
