import { describe, expect, it } from 'vitest';
import { mapNotificationPreferencesResponse } from './mappers';
import { NOTIFICATION_EVENT_TYPES } from './types';

describe('mapNotificationPreferencesResponse', () => {
  it('maps live event types into preferences', () => {
    const data = mapNotificationPreferencesResponse({
      preferences: [
        { event_type: 'operation_due', email_allowed: true, push_allowed: false },
      ],
    });

    expect(data.preferences).toEqual([
      { eventType: 'operation_due', emailAllowed: true, pushAllowed: false },
    ]);
    expect(data.carried).toEqual([]);
  });

  it('splits free_reminder into carried — the generated client still carries the dead enum value (ticket #381)', () => {
    const data = mapNotificationPreferencesResponse({
      preferences: [
        { event_type: 'free_reminder', email_allowed: false, push_allowed: true },
        { event_type: 'subscription_grace', email_allowed: false, push_allowed: false },
      ],
    });

    expect(data.preferences.map((preference) => preference.eventType)).toEqual([
      'subscription_grace',
    ]);
    expect(data.carried).toEqual([
      { eventType: 'free_reminder', emailAllowed: false, pushAllowed: true },
    ]);
  });

  it('the runtime event-type list no longer knows free_reminder', () => {
    expect(NOTIFICATION_EVENT_TYPES).not.toContain('free_reminder');
  });
});
