import type {
  NotificationEventType,
  NotificationPreference,
} from '@/entities/user';

/**
 * Каталог событий доставочных уведомлений. После удаления домена аренд
 * (спека #434) осталось единственное событие — grace-письма биллинга.
 */
export const NOTIFICATION_OPTIONS = [
  {
    eventType: 'subscription_grace',
    label: 'Оплата подписки',
    description: 'Неудачное списание за тариф и конец льготного периода',
  },
] as const satisfies ReadonlyArray<{
  readonly eventType: NotificationEventType;
  readonly label: string;
  readonly description: string;
}>;

/**
 * Per-event-type × per-channel state (ADR 0030). Экран настроек показывает
 * только email-чекбокс; push-флаг протаскивается с сервера нетронутым —
 * PUT требует полный набор пар (событие, канал).
 */
export type NotificationChannelState = Record<
  NotificationEventType,
  { readonly email: boolean; readonly push: boolean }
>;

/** Default opt-in state for both channels (ADR 0030 §1: opt-out default). */
const DEFAULT_CHANNEL_FLAGS = { email: true, push: true } as const;

/**
 * Build the channel-matrix state from the server preferences. A missing type
 * defaults to allowed on both channels.
 */
export function buildInitialChannelPreferences(
  preferences: NotificationPreference[],
): NotificationChannelState {
  const state: NotificationChannelState = {
    subscription_grace: { ...DEFAULT_CHANNEL_FLAGS },
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
 * One payload item for the PUT endpoint: an event type driven by the UI state.
 */
export type NotificationPreferencePayloadItem = NotificationPreference;

/**
 * Build the per-event-type × per-channel payload for the PUT endpoint from a
 * channel-matrix state. Бэкенд валидирует полноту: ровно по одной записи на
 * каждую пару (событие, канал).
 */
export function buildChannelPreferencePayload(
  state: NotificationChannelState,
): NotificationPreferencePayloadItem[] {
  return NOTIFICATION_OPTIONS.map(({eventType}) => {
    const flags = state[eventType];
    return {
      eventType,
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
