// Мэппинг DTO CalendarReminderItem → entity CalendarEntry и доменные лейблы/селекторы.
import type { components } from '@/shared/api/generated';
import type {
  CalendarEntry,
  CalendarEntryEventType,
  CalendarEntryPeriodicity,
  CalendarEntryStatus,
  CalendarEntryType,
} from './types';

type CalendarReminderItem = components['schemas']['CalendarReminderItem'];

export const ORPHAN_PROPERTY_NAME = 'Без объекта';

export const calendarEntryTypeLabels: Record<CalendarEntryType, string> = {
  free: 'Свободное',
  operation: 'По операции',
  system: 'Системное',
};

// Статус-чипы показываются только у operation/system. Лейблы по event_type
// (фактический повод), с акцентом на «Просрочено» красным.
export const calendarEntryStatusLabels: Record<CalendarEntryEventType, string> = {
  operation_due: 'Скоро срок',
  operation_overdue: 'Просрочено',
  lease_expiring: 'Окончание аренды',
  lease_requires_action: 'Требует действия',
  free_reminder: '',
};

export function mapCalendarEntry(dto: CalendarReminderItem): CalendarEntry {
  return {
    id: dto.id,
    type: dto.type,
    scheduledAt: dto.scheduled_at,
    title: dto.title,
    hasProperty: dto.has_property,
    propertyName: dto.property_name ?? null,
    status: (dto.status as CalendarEntryStatus | null | undefined) ?? null,
    eventType: (dto.event_type as CalendarEntryEventType | null | undefined) ?? null,
    periodicity: (dto.periodicity as CalendarEntryPeriodicity | null | undefined) ?? null,
    operationId: dto.operation_id ?? null,
    leaseId: dto.lease_id ?? null,
    freeReminderId: dto.free_reminder_id ?? null,
  };
}

// Маркер «↻» только для повторяющихся свободных напоминаний.
export function isRecurring(entry: CalendarEntry): boolean {
  return entry.periodicity !== null && entry.periodicity !== 'once';
}

// Статус-чип: null для свободных; для operation/system — по event_type.
export function entryStatusEventType(entry: CalendarEntry): CalendarEntryEventType | null {
  if (entry.type === 'free') {
    return null;
  }
  return entry.eventType;
}

export function propertyDisplayName(entry: CalendarEntry): string {
  return entry.propertyName ?? ORPHAN_PROPERTY_NAME;
}
