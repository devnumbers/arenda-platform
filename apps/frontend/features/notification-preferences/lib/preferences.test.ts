import { describe, expect, it } from 'vitest';
import {
  buildChannelPreferencePayload,
  buildInitialChannelPreferences,
  buildInitialPreferences,
  buildPreferencePayload,
  NOTIFICATION_OPTIONS,
  type NotificationChannelState,
  type NotificationPreferencesState,
} from './preferences';

describe('NOTIFICATION_OPTIONS', () => {
  it('no longer lists the removed Свои напоминания row (ticket #381)', () => {
    expect(NOTIFICATION_OPTIONS.map((option) => option.eventType)).not.toContain('free_reminder');
    expect(NOTIFICATION_OPTIONS.map((option) => option.label)).not.toContain('Свои напоминания');
  });
});

describe('buildInitialPreferences / buildInitialChannelPreferences', () => {
  it('default state has no free_reminder key', () => {
    const state = buildInitialPreferences([]);
    const channelState = buildInitialChannelPreferences([]);

    expect(Object.keys(state)).not.toContain('free_reminder');
    expect(Object.keys(channelState)).not.toContain('free_reminder');
  });
});

describe('buildPreferencePayload', () => {
  it('builds one payload item per live event type', () => {
    const state: NotificationPreferencesState = {
      operation_due: false,
      operation_overdue: true,
      lease_expiring: true,
      lease_requires_action: true,
      subscription_grace: true,
    };

    const payload = buildPreferencePayload(state, []);

    expect(payload).toHaveLength(NOTIFICATION_OPTIONS.length);
    expect(payload[0]).toEqual({
      eventType: 'operation_due',
      emailAllowed: false,
      pushAllowed: false,
    });
  });
});

describe('buildChannelPreferencePayload', () => {
  it('builds one payload item per live event type from the matrix flags', () => {
    const state: NotificationChannelState = {
      operation_due: { email: false, push: true },
      operation_overdue: { email: true, push: true },
      lease_expiring: { email: true, push: true },
      lease_requires_action: { email: true, push: true },
      subscription_grace: { email: true, push: true },
    };

    const payload = buildChannelPreferencePayload(state);

    expect(payload).toHaveLength(NOTIFICATION_OPTIONS.length);
    expect(payload[0]).toEqual({
      eventType: 'operation_due',
      emailAllowed: false,
      pushAllowed: true,
    });
  });
});
