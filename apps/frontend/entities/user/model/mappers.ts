import type { components } from '@/shared/api/dto';
import {
  NOTIFICATION_EVENT_TYPES,
  type CarriedNotificationPreference,
  type NotificationPreference,
  type User,
} from './types';

type MeResponse = components['schemas']['MeResponse'];
type NotificationPreferencesResponse =
  components['schemas']['NotificationPreferencesResponse'];

export function mapMeResponse(response: MeResponse): User {
  return {
    id: response.id,
    phone: response.phone,
    role: response.role,
    name: response.name ?? null,
    surname: response.surname ?? null,
    patronymic: response.patronymic ?? null,
    email: response.email ?? null,
    timezone: response.timezone ?? null,
    subscription: response.subscription
      ? {
          tariff: {
            name: response.subscription.tariff.name,
          },
        }
      : null,
  };
}

const knownEventTypes = new Set<string>(NOTIFICATION_EVENT_TYPES);

export type NotificationPreferencesData = {
  /** Живые события — drives the settings UI state. */
  readonly preferences: NotificationPreference[];
  /** Выведенные из продукта события — UI не показывает, PUT переносит как есть. */
  readonly carried: CarriedNotificationPreference[];
};

// Ответ сервера делится по живому списку событий: free_reminder (пока enum
// контракта его содержит, тикет #381) уходит в carried и не попадает в UI.
export function mapNotificationPreferencesResponse(
  response: NotificationPreferencesResponse,
): NotificationPreferencesData {
  const preferences: NotificationPreference[] = [];
  const carried: CarriedNotificationPreference[] = [];
  for (const preference of response.preferences) {
    if (knownEventTypes.has(preference.event_type)) {
      preferences.push({
        eventType: preference.event_type as NotificationPreference['eventType'],
        emailAllowed: preference.email_allowed,
        pushAllowed: preference.push_allowed,
      });
    } else {
      carried.push({
        eventType: preference.event_type as CarriedNotificationPreference['eventType'],
        emailAllowed: preference.email_allowed,
        pushAllowed: preference.push_allowed,
      });
    }
  }
  return { preferences, carried };
}
