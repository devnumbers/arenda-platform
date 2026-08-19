import { describe, expect, it } from 'vitest';
import type { components } from '@/shared/api/dto';
import { mapCalendarEntries } from './mapping';

type CalendarReminderItem = components['schemas']['CalendarReminderItem'];

function makeDto(
  overrides: Partial<CalendarReminderItem> = {},
): CalendarReminderItem {
  return {
    id: 'reminder-1',
    type: 'operation',
    scheduled_at: '2026-06-01T10:00:00Z',
    title: 'Оплата электричества',
    has_property: true,
    property_name: 'Квартира на Ленина',
    status: 'pending',
    event_type: 'operation_due',
    operation_id: 'operation-1',
    ...overrides,
  };
}

describe('mapCalendarEntries', () => {
  it('maps operation and system entries', () => {
    const entries = mapCalendarEntries([
      makeDto(),
      makeDto({
        id: 'reminder-2',
        type: 'system',
        event_type: 'lease_expiring',
        operation_id: null,
        lease_id: 'lease-1',
      }),
    ]);

    expect(entries).toHaveLength(2);
    expect(entries[0]).toEqual({
      id: 'reminder-1',
      type: 'operation',
      scheduledAt: '2026-06-01T10:00:00Z',
      title: 'Оплата электричества',
      hasProperty: true,
      propertyName: 'Квартира на Ленина',
      status: 'pending',
      eventType: 'operation_due',
      operationId: 'operation-1',
      leaseId: null,
    });
  });

  it('drops free entries — the generated client still carries the dead type (ticket #381)', () => {
    const entries = mapCalendarEntries([
      makeDto({
        id: 'free-1',
        type: 'free',
        event_type: 'free_reminder',
        operation_id: null,
        periodicity: 'weekly',
        free_reminder_id: 'template-1',
      }),
      makeDto(),
    ]);

    expect(entries.map((entry) => entry.id)).toEqual(['reminder-1']);
  });
});
