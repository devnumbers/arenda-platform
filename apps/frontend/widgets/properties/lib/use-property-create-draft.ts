'use client';

import { useState, useEffect } from 'react';
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

export function usePropertyCreateDraft() {
  const [draft, setDraft] = useState<CreateDraft>(() => loadDraft());

  useEffect(() => {
    if (draft.step === 4) {
      sessionStorage.removeItem(STORAGE_KEY);
      return;
    }
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify(draft));
  }, [draft]);

  return { draft, setDraft };
}

function loadDraft(): CreateDraft {
  if (typeof window === 'undefined') return { step: 1 };
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY);
    if (!raw) return { step: 1 };
    const parsed = JSON.parse(raw) as CreateDraft;
    if (parsed.step < 1 || parsed.step > 4) return { step: 1 };
    return parsed;
  } catch {
    return { step: 1 };
  }
}
