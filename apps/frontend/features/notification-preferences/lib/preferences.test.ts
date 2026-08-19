import { describe, expect, it } from 'vitest';
import type { CarriedNotificationPreference } from '@/entities/user';
import {
  buildChannelPreferencePayload,
  buildInitialChannelPreferences,
  buildInitialPreferences,
  buildPreferencePayload,
  NOTIFICATION_OPTIONS,
  type NotificationChannelState,
  type NotificationPreferencesState,
} from './preferences';

const carried: readonly CarriedNotificationPreference[] = [
  { eventType: 'free_reminder', emailAllowed: false, pushAllowed: true },
];

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
  it('appends carried rows unchanged so the removed event keeps its server values', () => {
    const state: NotificationPreferencesState = {
      operation_due: false,
      operation_overdue: true,
      lease_expiring: true,
      lease_requires_action: true,
      subscription_grace: true,
    };

    const payload = buildPreferencePayload(state, [], carried);

    expect(payload).toHaveLength(NOTIFICATION_OPTIONS.length + carried.length);
    expect(payload[payload.length - 1]).toEqual(carried[0]);
    expect(payload[0]).toEqual({
      eventType: 'operation_due',
      emailAllowed: false,
      pushAllowed: false,
    });
  });
});

describe('buildChannelPreferencePayload', () => {
  it('appends carried rows unchanged so the PUT keeps the full event set', () => {
    const state: NotificationChannelState = {
      operation_due: { email: false, push: true },
      operation_overdue: { email: true, push: true },
      lease_expiring: { email: true, push: true },
      lease_requires_action: { email: true, push: true },
      subscription_grace: { email: true, push: true },
    };

    const payload = buildChannelPreferencePayload(state, carried);

    expect(payload).toHaveLength(NOTIFICATION_OPTIONS.length + carried.length);
    expect(payload[payload.length - 1]).toEqual(carried[0]);
    expect(payload[0]).toEqual({
      eventType: 'operation_due',
      emailAllowed: false,
      pushAllowed: true,
    });
  });
});
