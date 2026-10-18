'use client';

import type { Dispatch, SetStateAction } from 'react';
import { useDraftStore } from '@/shared/lib/hooks/useDraftStore';
import { isEmailValid } from '@/shared/lib/email';

export type LoginStep = 'email' | 'code';

export type LoginDraft = {
  step: LoginStep;
  email: string;
};

const STORAGE_KEY = 'login-draft';

const DEFAULT_DRAFT: LoginDraft = {
  step: 'email',
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
    validate: validateLoginDraft,
  });
  return { draft, setDraft, clearDraft };
}

function isString(value: unknown): value is string {
  return typeof value === 'string';
}

function isStep(value: unknown): value is LoginStep {
  return value === 'email' || value === 'code';
}

/** Нормализация payload'а из хранилища: шаг кода живёт только с почтой,
 * похожей на адрес; иначе рестарт на шаг почты — набранное не стирается.
 * Телефонный черновик прежней версии ({step: 'phone', …}) не матчится
 * ни одним шагом и роняется в дефолт (телефон покинул вход, ADR 0067). */
export function validateLoginDraft(parsed: unknown): LoginDraft {
  if (parsed === null || typeof parsed !== 'object') return DEFAULT_DRAFT;

  const record = parsed as Record<string, unknown>;

  if (!isStep(record.step)) return DEFAULT_DRAFT;
  if (!isString(record.email)) return DEFAULT_DRAFT;

  const step: LoginStep = record.step !== 'email' && !isEmailValid(record.email) ? 'email' : record.step;

  return {
    step,
    email: record.email,
  };
}
