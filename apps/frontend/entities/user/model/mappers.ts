import type { components } from '@/shared/api/generated';
import type { NotificationPreference, User } from './types';

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
    subscription: response.subscription
      ? {
          tariff: {
            name: response.subscription.tariff.name,
          },
        }
      : null,
  };
}

export function mapNotificationPreferencesResponse(
  response: NotificationPreferencesResponse,
): NotificationPreference[] {
  return response.preferences.map((preference) => ({
    eventType: preference.event_type,
    allowed: preference.allowed,
  }));
}
