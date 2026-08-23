import type {
  OperationFrequency,
  OperationType,
} from '@/entities/operation';

export type BasicInfoData = {
  amount: string;
  name: string;
  /** Category id (uuid). */
  category?: string;
  propertyId?: string;
  comment?: string;
  type?: OperationType;
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
  endDate?: string;
};

export type ScheduleErrors = {
  frequency?: string;
  date?: string;
  endDate?: string;
};

export type ReminderData = {
  enabled: boolean;
  offsetDays: 1 | 3 | 7;
};

export type { OperationType };
