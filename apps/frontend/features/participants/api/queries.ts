import { queryOptions, type UseQueryOptions } from '@tanstack/react-query';
import { apiClient, type ApiTransport } from '@/shared/api/client';
import type { ApiError } from '@/shared/api/errors';
import type { components } from '@/shared/api/dto';
import { participantsKeys } from '@/shared/api/query-keys';
import { mapParticipant } from '@/entities/participants';
import type { Participant } from '@/entities/participants';

/** Двойной модуль API-слоя participants (без 'use client'): чистые
 * fetch-функции и queryOptions-фабрики канона #887 — общий источник
 * ключ+fetch для клиентских хуков и серверного префетча. Хуки — в
 * hooks.ts ('use client'). */

/** Чистый fetch страницы участника — общее горло хука и серверного
 * префетча #887. */
export async function fetchParticipant(
  participantId: string,
  transport: ApiTransport = apiClient,
): Promise<Participant> {
  const response = await transport<components['schemas']['ParticipantResponse']>(
    `/participants/${encodeURIComponent(participantId)}`,
  );
  return mapParticipant(response);
}

/** Опции страницы участника (канон #887): один источник ключ+fetch для
 * хука и серверного префетча. */
export function participantQueryOptions({
  participantId,
  transport = apiClient,
}: {
  readonly participantId: string;
  readonly transport?: ApiTransport;
}): UseQueryOptions<Participant, ApiError, Participant, ReturnType<typeof participantsKeys.detail>> {
  return queryOptions({
    queryKey: participantsKeys.detail(participantId),
    queryFn: () => fetchParticipant(participantId, transport),
  });
}
