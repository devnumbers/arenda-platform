'use client';

import type { Dispatch, SetStateAction } from 'react';
import { clearDraftStorage, useDraftStore } from '@/shared/lib/hooks/useDraftStore';
import type { PropertyAttributes, PropertyType } from '@/entities/property';
import { coerceAttributes } from '@/entities/property';
import { propertyTypeOptions } from '@/features/properties';

// Step 1 — Тип, 2 — Адрес, 3 — Характеристики, 4 — Информация, 5 — Success.
export type CreateStep = 1 | 2 | 3 | 4 | 5;

export type CreateDraft = {
  step: CreateStep;
  type?: PropertyType;
  address?: string;
  attributes?: PropertyAttributes;
  name?: string;
  description?: string;
};

const STORAGE_KEY = 'property-create-draft';

const DEFAULT_DRAFT: CreateDraft = { step: 1 };

export function clearPropertyCreateDraft(): void {
  clearDraftStorage(STORAGE_KEY);
}

export function usePropertyCreateDraft(): {
  draft: CreateDraft;
  setDraft: Dispatch<SetStateAction<CreateDraft>>;
} {
  const { draft, setDraft } = useDraftStore<CreateDraft>({
    storageKey: STORAGE_KEY,
    createDefault: () => DEFAULT_DRAFT,
    validate: validateDraft,
    isTerminal: (draft) => draft.step === 5,
  });
  return { draft, setDraft };
}

function isOptionalString(value: unknown): value is string | undefined {
  return value === undefined || typeof value === 'string';
}

function validateDraft(parsed: unknown): CreateDraft {
  if (parsed === null || typeof parsed !== 'object') return DEFAULT_DRAFT;

  const record = parsed as Record<string, unknown>;

  const step = Number(record.step);
  if (!Number.isInteger(step) || step < 1 || step > 5) return DEFAULT_DRAFT;

  if (
    'type' in record &&
    record.type !== undefined &&
    !propertyTypeOptions.some((option) => option.value === record.type)
  ) {
    return DEFAULT_DRAFT;
  }
  if (!isOptionalString(record.address)) return DEFAULT_DRAFT;
  const attributesPresent = 'attributes' in record && record.attributes !== undefined;
  if (!isOptionalString(record.name)) return DEFAULT_DRAFT;
  if (!isOptionalString(record.description)) return DEFAULT_DRAFT;

  return {
    step: step as CreateStep,
    ...(record.type !== undefined && { type: record.type as PropertyType }),
    ...(record.address !== undefined && { address: record.address }),
    ...(attributesPresent && { attributes: coerceAttributes(record.attributes) }),
    ...(record.name !== undefined && { name: record.name }),
    ...(record.description !== undefined && {
      description: record.description,
    }),
  };
}
