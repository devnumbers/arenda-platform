'use client';

import type { Dispatch, SetStateAction } from 'react';
import { useDraftStore } from '@/shared/lib/hooks/useDraftStore';
import type { OperationFrequency, OperationType } from '@/entities/operation';
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
  const { draft, setDraft } = useDraftStore<OperationCreateDraft>({
    storageKey: STORAGE_KEY,
    // The config freezes on first render, so the default keeps the entry
    // (type, propertyId) the wizard was opened with.
    createDefault: () => buildDefaultDraft(type, propertyId),
    validate: (parsed) => {
      const loaded = validateDraft(parsed, buildDefaultDraft(type, propertyId));
      // An explicit propertyId (from the URL) wins over the persisted draft; all
      // other persisted fields are kept as-is.
      return propertyId
        ? { ...loaded, basicInfo: { ...loaded.basicInfo, propertyId } }
        : loaded;
    },
    isTerminal: (draft) => draft.step === 'success',
  });
  return { draft, setDraft };
}

function isObject(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object';
}

function isOptionalString(value: unknown): value is string | undefined {
  return value === undefined || typeof value === 'string';
}

const UUID_PATTERN =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

function isUuid(value: string): boolean {
  return UUID_PATTERN.test(value);
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

  // Categories are per-user entities identified by uuid; discard stale enum
  // values (e.g. 'rent') persisted by older drafts.
  const category = basicInfo.category;
  if (category !== undefined && !isUuid(category)) {
    return {
      ...(parsed as unknown as OperationCreateDraft),
      basicInfo: { ...(basicInfo as unknown as BasicInfoData), category: undefined },
    };
  }

  return parsed as unknown as OperationCreateDraft;
}
