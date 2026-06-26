import type {
  OperationCategory,
  OperationFrequency,
  OperationType,
} from '@/entities/operation/model/types';

export type BasicInfoData = {
  amount: string;
  name: string;
  category?: OperationCategory;
  propertyId?: string;
  comment?: string;
};

export type BasicInfoErrors = {
  amount?: string;
  name?: string;
  category?: string;
  property?: string;
  comment?: string;
};

export type ScheduleData = {
  frequency: OperationFrequency;
  date?: string;
  paymentDay?: number;
  endDate?: string;
};

export type ScheduleErrors = {
  frequency?: string;
  date?: string;
  paymentDay?: string;
  endDate?: string;
};

export type ReminderData = {
  enabled: boolean;
  offsetDays: 1 | 3 | 7;
};

export type { OperationType };

export function toKopecks(amount: string): number | undefined {
  const normalized = amount.trim().replace(',', '.');
  if (normalized === '') {
    return undefined;
  }

  const value = Number(normalized);
  if (Number.isNaN(value) || value < 0) {
    return undefined;
  }

  return Math.round(value * 100);
}
