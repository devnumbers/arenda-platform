'use client';

import type { Dispatch, SetStateAction } from 'react';
import { clearDraftStorage, useDraftStore } from '@/shared/lib/hooks/useDraftStore';
import {
  DEFAULT_PROPERTY_CREATE_DRAFT,
  PROPERTY_CREATE_DRAFT_STORAGE_KEY,
  validatePropertyCreateDraft,
  type PropertyCreateDraft,
} from './property-create-draft';

/** Черновик флоу «Создание объекта» (#480): переживает перезагрузку
 * страницы и закрытие визарда — sessionStorage (как в старом визарде,
 * ключ тот же). Обёртка владеет только ключом, дефолтом и проверкой
 * формы — сам цикл хранения живёт в useDraftStore. */

export function clearPropertyCreateDraft(): void {
  clearDraftStorage(PROPERTY_CREATE_DRAFT_STORAGE_KEY);
}

export function usePropertyCreateDraft(): {
  readonly draft: PropertyCreateDraft;
  readonly isLoaded: boolean;
  readonly setDraft: Dispatch<SetStateAction<PropertyCreateDraft>>;
  readonly clearDraft: () => void;
} {
  return useDraftStore<PropertyCreateDraft>({
    storageKey: PROPERTY_CREATE_DRAFT_STORAGE_KEY,
    createDefault: () => DEFAULT_PROPERTY_CREATE_DRAFT,
    validate: validatePropertyCreateDraft,
  });
}
