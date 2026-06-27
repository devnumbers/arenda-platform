import type { components } from '@/shared/api/generated';
import type { User } from './types';

type MeResponse = components['schemas']['MeResponse'];

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
