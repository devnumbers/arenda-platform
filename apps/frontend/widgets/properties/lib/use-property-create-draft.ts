'use client';

import { useState, useEffect } from 'react';
import type { Dispatch, SetStateAction } from 'react';
import type { PropertyType } from '@/entities/property/model/types';

export type CreateStep = 1 | 2 | 3 | 4;

export type CreateDraft = {
  step: CreateStep;
  type?: PropertyType;
  address?: string;
  name?: string;
  description?: string;
};

const STORAGE_KEY = 'property-create-draft';

const DEFAULT_DRAFT: CreateDraft = { step: 1 };

export function usePropertyCreateDraft(): {
  draft: CreateDraft;
  setDraft: Dispatch<SetStateAction<CreateDraft>>;
} {
  const [draft, setDraft] = useState<CreateDraft>(DEFAULT_DRAFT);

  useEffect(() => {
    // Load persisted draft after hydration; reading sessionStorage during
    // render would cause an SSR/hydration mismatch in the App Router.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setDraft(loadDraft());
  }, []);

  useEffect(() => {
    try {
      if (draft.step === 4) {
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

function loadDraft(): CreateDraft {
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

function validateDraft(parsed: unknown): CreateDraft {
  if (parsed === null || typeof parsed !== 'object') return DEFAULT_DRAFT;

  const record = parsed as Record<string, unknown>;

  const step = Number(record.step);
  if (!Number.isInteger(step) || step < 1 || step > 4) return DEFAULT_DRAFT;

  if (
    'type' in record &&
    record.type !== undefined &&
    typeof record.type !== 'string'
  ) {
    return DEFAULT_DRAFT;
  }
  if (
    'address' in record &&
    record.address !== undefined &&
    typeof record.address !== 'string'
  ) {
    return DEFAULT_DRAFT;
  }
  if (
    'name' in record &&
    record.name !== undefined &&
    typeof record.name !== 'string'
  ) {
    return DEFAULT_DRAFT;
  }
  if (
    'description' in record &&
    record.description !== undefined &&
    typeof record.description !== 'string'
  ) {
    return DEFAULT_DRAFT;
  }

  return {
    step: step as CreateStep,
    ...(record.type !== undefined && { type: record.type as PropertyType }),
    ...(record.address !== undefined && { address: record.address as string }),
    ...(record.name !== undefined && { name: record.name as string }),
    ...(record.description !== undefined && {
      description: record.description as string,
    }),
  };
}
