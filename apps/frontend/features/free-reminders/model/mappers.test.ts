import { describe, expect, it } from 'vitest';
import {
  mapFreeReminderResponse,
  mapUpcomingFreeReminderResponse,
} from './mappers';

describe('mapFreeReminderResponse', () => {
  it('maps snake_case DTO to camelCase entity', () => {
    const reminder = mapFreeReminderResponse({
      id: 'reminder-1',
      owner_id: 'owner-1',
      property_id: 'property-1',
      title: 'Оплатить интернет',
      trigger_at: '2026-09-01T10:00:00Z',
      periodicity: 'monthly',
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
    });

    expect(reminder).toEqual({
      id: 'reminder-1',
      ownerId: 'owner-1',
      propertyId: 'property-1',
      title: 'Оплатить интернет',
      triggerAt: '2026-09-01T10:00:00Z',
      periodicity: 'monthly',
      createdAt: '2026-01-01T00:00:00Z',
      updatedAt: '2026-01-01T00:00:00Z',
    });
  });
});

describe('mapUpcomingFreeReminderResponse', () => {
  it('maps free_reminder_id to freeReminderId', () => {
    const upcoming = mapUpcomingFreeReminderResponse({
      free_reminder_id: 'reminder-1',
      title: 'Оплатить интернет',
      property_id: 'property-1',
      trigger_at: '2026-09-01T10:00:00Z',
      periodicity: 'once',
    });

    expect(upcoming).toEqual({
      freeReminderId: 'reminder-1',
      title: 'Оплатить интернет',
      propertyId: 'property-1',
      triggerAt: '2026-09-01T10:00:00Z',
      periodicity: 'once',
    });
  });
});
