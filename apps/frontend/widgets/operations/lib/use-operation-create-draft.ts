'use client';

import { useState, useEffect } from 'react';
import type { Dispatch, SetStateAction } from 'react';
import type { OperationFrequency, OperationType } from '@/entities/operation/model/types';
import type { BasicInfoData, ReminderData, ScheduleData } from '../model/types';

export type OperationCreateStep = 'basic' | 'schedule' | 'reminder' | 'success';

export type OperationCreateDraft = {
  step: OperationCreateStep;
  operationType: OperationType;
  basicInfo: BasicInfoData;
  schedule: ScheduleData;
  reminder: ReminderData;
};

const STORAGE_KEY = 'operation-create-draft';

const STEPS: OperationCreateStep[] = ['basic', 'schedule', 'reminder', 'success'];
const OPERATION_TYPES: OperationType[] = ['income', 'expense'];
const FREQUENCIES: OperationFrequency[] = ['once', 'monthly', 'yearly'];
const REMINDER_OFFSETS = [1, 3, 7] as const;

function buildDefaultDraft(type: OperationType, propertyId?: string): OperationCreateDraft {
  return {
    step: 'basic',
    operationType: type,
    basicInfo: {
      amount: '',
      name: '',
      category: undefined,
      propertyId: propertyId ?? undefined,
      comment: '',
      type,
    },
    schedule: { frequency: 'once', date: undefined, endDate: undefined },
    reminder: { enabled: false, offsetDays: 1 },
  };
}

export function useOperationCreateDraft(
  type: OperationType,
  propertyId?: string,
): {
  draft: OperationCreateDraft;
  setDraft: Dispatch<SetStateAction<OperationCreateDraft>>;
} {
  const [defaultDraft] = useState<OperationCreateDraft>(() => buildDefaultDraft(type, propertyId));
  const [draft, setDraft] = useState<OperationCreateDraft>(defaultDraft);

  useEffect(() => {
    // Load persisted draft after hydration; reading sessionStorage during render would cause an SSR/hydration mismatch.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setDraft(loadDraft(defaultDraft));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    try {
      if (draft.step === 'success') {
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

function loadDraft(defaultDraft: OperationCreateDraft): OperationCreateDraft {
  if (typeof window === 'undefined') return defaultDraft;

  try {
    const raw = sessionStorage.getItem(STORAGE_KEY);
    if (!raw) return defaultDraft;
    const parsed = JSON.parse(raw) as unknown;
    return validateDraft(parsed, defaultDraft);
  } catch {
    return defaultDraft;
  }
}

function isObject(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object';
}

function isOptionalString(value: unknown): value is string | undefined {
  return value === undefined || typeof value === 'string';
}

function validateDraft(parsed: unknown, defaultDraft: OperationCreateDraft): OperationCreateDraft {
  if (!isObject(parsed)) return defaultDraft;

  const step = parsed.step;
  if (typeof step !== 'string' || !STEPS.includes(step as OperationCreateStep)) {
    return defaultDraft;
  }

  const operationType = parsed.operationType;
  if (typeof operationType !== 'string' || !OPERATION_TYPES.includes(operationType as OperationType)) {
    return defaultDraft;
  }

  if (!isObject(parsed.basicInfo)) return defaultDraft;
  const basicInfo = parsed.basicInfo;
  if (!isOptionalString(basicInfo.amount)) return defaultDraft;
  if (!isOptionalString(basicInfo.name)) return defaultDraft;
  if (!isOptionalString(basicInfo.comment)) return defaultDraft;
  if (!isOptionalString(basicInfo.category)) return defaultDraft;
  if (!isOptionalString(basicInfo.propertyId)) return defaultDraft;
  if (!isOptionalString(basicInfo.type)) return defaultDraft;

  if (!isObject(parsed.schedule)) return defaultDraft;
  const schedule = parsed.schedule;
  if (typeof schedule.frequency !== 'string' || !FREQUENCIES.includes(schedule.frequency as OperationFrequency)) {
    return defaultDraft;
  }
  if (!isOptionalString(schedule.date)) return defaultDraft;
  if (!isOptionalString(schedule.endDate)) return defaultDraft;

  if (!isObject(parsed.reminder)) return defaultDraft;
  const reminder = parsed.reminder;
  if (typeof reminder.enabled !== 'boolean') return defaultDraft;
  if (
    typeof reminder.offsetDays !== 'number' ||
    !REMINDER_OFFSETS.includes(reminder.offsetDays as 1 | 3 | 7)
  ) {
    return defaultDraft;
  }

  return parsed as unknown as OperationCreateDraft;
}
