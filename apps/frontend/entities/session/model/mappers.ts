import type { components } from '@/shared/api/dto';
import { type ActiveSession } from './types';

type SessionListResponse = components['schemas']['SessionListResponse'];
type ActiveSessionDto = components['schemas']['ActiveSession'];

/** DTO→entity списка активных сессий (GET /me/sessions, #728): порядок
 * бэка (по свежей активности) сохраняется. Опциональные поля DTO
 * (`browserMajor`, `city`, `lastIp`) нормализуются в null. */
export function mapSessionListResponse(response: SessionListResponse): ActiveSession[] {
  return response.sessions.map((session: ActiveSessionDto) => ({
    id: session.id,
    deviceType: session.deviceType,
    browser: session.browser,
    browserMajor: session.browserMajor ?? null,
    os: session.os,
    city: session.city ?? null,
    lastIp: session.lastIp ?? null,
    lastSeenAt: session.lastSeenAt,
    createdAt: session.createdAt,
    current: session.current,
  }));
}
