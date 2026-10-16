import { queryOptions, type UseQueryOptions } from '@tanstack/react-query';
import { apiClient, type ApiTransport } from '@/shared/api/client';
import type { ApiError } from '@/shared/api/errors';
import { authKeys } from '@/shared/api/query-keys';
import type { components } from '@/shared/api/dto';
import type { User } from '@/entities/user';
import { mapMeResponse } from '@/entities/user';

type MeResponse = components['schemas']['MeResponse'];

/** Двойной модуль API-слоя auth (без 'use client'): чистые fetch-функции и
 * queryOptions-фабрики канона #887 — общий источник ключ+fetch для
 * клиентских хуков и серверного префетча. Хуки — в hooks.ts ('use client'). */

/** Чистый fetch /me — общее горло хука и серверного префетча #887. */
export async function fetchMe(transport: ApiTransport = apiClient): Promise<User> {
  const res = await transport<MeResponse>('/me');
  return mapMeResponse(res);
}

/** Опции /me (канон #887): один источник ключ+fetch для хука и серверного
 * префетча. */
export function meQueryOptions(
  transport: ApiTransport = apiClient,
): UseQueryOptions<User, ApiError, User, typeof authKeys.me> {
  return queryOptions({
    queryKey: authKeys.me,
    queryFn: () => fetchMe(transport),
  });
}
