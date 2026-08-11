import type {
  NotificationEventType,
  NotificationPreference,
} from '@/entities/user/model/types';

export const NOTIFICATION_OPTIONS: {
  readonly eventType: NotificationEventType;
  readonly label: string;
  readonly description: string;
}[] = [
  {
    eventType: 'operation_due',
    label: 'Предстоящие операции',
    description: 'За 1, 3 или 7 дней до операции; срок выбирается в самой операции',
  },
  {
    eventType: 'operation_overdue',
    label: 'Просроченные операции',
    description: 'Если операция не отмечена вовремя',
  },
  {
    eventType: 'lease_expiring',
    label: 'Окончание аренды',
    description: 'За 30 дней до конца аренды',
  },
  {
    eventType: 'lease_requires_action',
    label: 'Аренда требует действия',
    description: 'Аренда закончилась и ждёт решения',
  },
  {
    eventType: 'free_reminder',
    label: 'Свои напоминания',
    description: 'Созданные вами напоминания о любых датах по объектам',
  },
];

export type NotificationPreferencesState = Record<NotificationEventType, boolean>;

/**
 * Per-event-type × per-channel state used by the two-column matrix in
 * Профиль → Уведомления (ADR 0030). Each event type carries independent
 * `email` and `push` flags.
 */
export type NotificationChannelState = Record<
  NotificationEventType,
  { readonly email: boolean; readonly push: boolean }
>;

/** Default opt-in state for both channels (ADR 0030 §1: opt-out default). */
const DEFAULT_CHANNEL_FLAGS = { email: true, push: true } as const;

export function buildInitialPreferences(
  preferences: NotificationPreference[],
): NotificationPreferencesState {
  const state: NotificationPreferencesState = {
    operation_due: true,
    operation_overdue: true,
    lease_expiring: true,
    lease_requires_action: true,
    free_reminder: true,
  };
  for (const preference of preferences) {
    state[preference.eventType] = preference.allowed;
  }
  return state;
}

/**
 * Build the channel-matrix state from the server preferences. Missing types
 * default to allowed on both channels.
 */
export function buildInitialChannelPreferences(
  preferences: NotificationPreference[],
): NotificationChannelState {
  const state: NotificationChannelState = {
    operation_due: { ...DEFAULT_CHANNEL_FLAGS },
    operation_overdue: { ...DEFAULT_CHANNEL_FLAGS },
    lease_expiring: { ...DEFAULT_CHANNEL_FLAGS },
    lease_requires_action: { ...DEFAULT_CHANNEL_FLAGS },
    free_reminder: { ...DEFAULT_CHANNEL_FLAGS },
  };
  for (const preference of preferences) {
    state[preference.eventType] = {
      email: preference.emailAllowed,
      push: preference.pushAllowed,
    };
  }
  return state;
}

/**
 * Builds the full per-event-type payload for the PUT endpoint from a UI state.
 * The expand-phase UI (ADR 0030) exposes a single toggle per event type that
 * drives the email channel; push settings are carried through from the last
 * known server preferences so they are not clobbered on save.
 */
export function buildPreferencePayload(
  state: NotificationPreferencesState,
  preferences: NotificationPreference[],
): NotificationPreference[] {
  const pushAllowedByType = new Map<NotificationEventType, boolean>();
  for (const preference of preferences) {
    pushAllowedByType.set(preference.eventType, preference.pushAllowed);
  }
  return NOTIFICATION_OPTIONS.map(({eventType}) => {
    const allowed = state[eventType];
    const pushAllowed = pushAllowedByType.get(eventType) ?? allowed;
    return {
      eventType,
      allowed,
      emailAllowed: allowed,
      pushAllowed,
    };
  });
}

/**
 * Build the per-event-type × per-channel payload for the PUT endpoint from a
 * channel-matrix state. `email_allowed` drives the legacy `allowed` field
 * (ADR 0030 §4: `allowed` equals `email_allowed` during the expand phase).
 */
export function buildChannelPreferencePayload(
  state: NotificationChannelState,
): NotificationPreference[] {
  return NOTIFICATION_OPTIONS.map(({eventType}) => {
    const flags = state[eventType];
    return {
      eventType,
      allowed: flags.email,
      emailAllowed: flags.email,
      pushAllowed: flags.push,
    };
  });
}

/** Structural equality over the fixed event-type set (both channels). */
export function channelPreferencesEqual(
  a: NotificationChannelState,
  b: NotificationChannelState,
): boolean {
  return NOTIFICATION_OPTIONS.every(
    (option) =>
      a[option.eventType].email === b[option.eventType].email &&
      a[option.eventType].push === b[option.eventType].push,
  );
}
