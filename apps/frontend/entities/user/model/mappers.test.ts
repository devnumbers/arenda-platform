import { describe, expect, it } from 'vitest';
import { mapNotificationPreferencesResponse } from './mappers';

describe('mapNotificationPreferencesResponse', () => {
  it('maps every event type row into preferences', () => {
    const preferences = mapNotificationPreferencesResponse({
      preferences: [
        { event_type: 'subscription_grace', email_allowed: true, push_allowed: false },
      ],
    });

    expect(preferences).toEqual([
      { eventType: 'subscription_grace', emailAllowed: true, pushAllowed: false },
    ]);
  });
});
