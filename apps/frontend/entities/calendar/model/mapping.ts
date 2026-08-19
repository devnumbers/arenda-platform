// Мэппинг DTO CalendarReminderItem → entity CalendarEntry и доменные лейблы/селекторы.
import type { components } from '@/shared/api/dto';
import type {
  CalendarEntry,
  CalendarEntryEventType,
  CalendarEntryStatus,
  CalendarEntryType,
} from './types';

type CalendarReminderItem = components['schemas']['CalendarReminderItem'];

export const ORPHAN_PROPERTY_NAME = 'Без объекта';

export const calendarEntryTypeLabels: Record<CalendarEntryType, string> = {
  operation: 'По операции',
  system: 'Системное',
};

// Статус-чипы показываются у operation/system. Лейблы по event_type
// (фактический повод), с акцентом на «Просрочено» красным.
export const calendarEntryStatusLabels: Record<CalendarEntryEventType, string> = {
  operation_due: 'Скоро срок',
  operation_overdue: 'Просрочено',
  lease_expiring: 'Окончание аренды',
  lease_requires_action: 'Требует действия',
};

export function mapCalendarEntries(
  dtos: readonly CalendarReminderItem[],
): CalendarEntry[] {
  return dtos.map((dto) => ({
    id: dto.id,
    type: dto.type,
    scheduledAt: dto.scheduled_at,
    title: dto.title,
    hasProperty: dto.has_property,
    propertyName: dto.property_name ?? null,
    status: (dto.status as CalendarEntryStatus | null | undefined) ?? null,
    eventType: (dto.event_type as CalendarEntryEventType | null | undefined) ?? null,
    operationId: dto.operation_id ?? null,
    leaseId: dto.lease_id ?? null,
  }));
}

export function propertyDisplayName(entry: CalendarEntry): string {
  return entry.propertyName ?? ORPHAN_PROPERTY_NAME;
}
