import { describe, expect, it } from 'vitest';
import { mapNotificationPreferencesResponse } from './mappers';

describe('mapNotificationPreferencesResponse', () => {
  it('maps every event type row into preferences', () => {
    const preferences = mapNotificationPreferencesResponse({
      preferences: [
        { event_type: 'operation_due', email_allowed: true, push_allowed: false },
        { event_type: 'subscription_grace', email_allowed: false, push_allowed: false },
      ],
    });

    expect(preferences).toEqual([
      { eventType: 'operation_due', emailAllowed: true, pushAllowed: false },
      { eventType: 'subscription_grace', emailAllowed: false, pushAllowed: false },
    ]);
  });
});
