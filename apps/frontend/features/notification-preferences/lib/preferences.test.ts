import { describe, expect, it } from 'vitest';
import {
  buildChannelPreferencePayload,
  buildInitialChannelPreferences,
  channelPreferencesEqual,
  NOTIFICATION_OPTIONS,
  type NotificationChannelState,
} from './preferences';

describe('NOTIFICATION_OPTIONS', () => {
  it('lists only subscription_grace after the leases removal (ticket #439)', () => {
    expect(NOTIFICATION_OPTIONS.map((option) => option.eventType)).toEqual([
      'subscription_grace',
    ]);
  });
});

describe('buildInitialChannelPreferences', () => {
  it('defaults a missing row to allowed on both channels', () => {
    expect(buildInitialChannelPreferences([])).toEqual({
      subscription_grace: { email: true, push: true },
    });
  });

  it('overlays the stored row on the default', () => {
    expect(
      buildInitialChannelPreferences([
        { eventType: 'subscription_grace', emailAllowed: false, pushAllowed: true },
      ]),
    ).toEqual({
      subscription_grace: { email: false, push: true },
    });
  });
});

describe('buildChannelPreferencePayload', () => {
  it('builds the full (event, channel) set required by the PUT endpoint', () => {
    const state: NotificationChannelState = {
      subscription_grace: { email: false, push: true },
    };

    expect(buildChannelPreferencePayload(state)).toEqual([
      { eventType: 'subscription_grace', emailAllowed: false, pushAllowed: true },
    ]);
  });
});

describe('channelPreferencesEqual', () => {
  it('detects an email-only difference', () => {
    const a: NotificationChannelState = {
      subscription_grace: { email: true, push: true },
    };
    const b: NotificationChannelState = {
      subscription_grace: { email: false, push: true },
    };
    expect(channelPreferencesEqual(a, a)).toBe(true);
    expect(channelPreferencesEqual(a, b)).toBe(false);
  });
});
