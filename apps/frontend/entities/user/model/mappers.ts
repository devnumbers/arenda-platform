import type { components } from '@/shared/api/dto';
import type { User } from './types';

type MeResponse = components['schemas']['MeResponse'];

export function mapMeResponse(response: MeResponse): User {
  return {
    id: response.id,
    // Телефон покинул /me (ADR 0067, миграция 000162): переходное значение
    // до тикета #1530 — он сносит поле вместе с телефонными строками
    // профиля (Экран 2 карты #1525); сейчас гасит компилятор, не роняя
    // виджеты профиля, которые поле ещё читают.
    phone: "",
    role: response.role,
    name: response.name ?? null,
    surname: response.surname ?? null,
    patronymic: response.patronymic ?? null,
    email: response.email ?? null,
    timezone: response.timezone ?? null,
    photoUrl: response.photoUrl ?? null,
    subscription: response.subscription
      ? {
          tariff: {
            name: response.subscription.tariff.name,
          },
        }
      : null,
  };
}
