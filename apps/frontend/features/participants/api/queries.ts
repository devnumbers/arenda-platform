import { queryOptions, type UseQueryOptions } from '@tanstack/react-query';
import { apiClient, type ApiTransport } from '@/shared/api/client';
import type { ApiError } from '@/shared/api/errors';
import type { components } from '@/shared/api/dto';
import { participantsKeys } from '@/shared/api/query-keys';
import { mapParticipant } from '@/entities/participants';
import type { Participant } from '@/entities/participants';

/** Счётчики хаба «Совместный доступ» (#693, GET /participants/summary):
 * участники читающего (агрегат members∪invitations его скоупа) и чужие
 * объекты, к которым у него есть активный доступ. */
export type ParticipantsSummary = {
  readonly participantsCount: number;
  readonly accessiblePropertiesCount: number;
};

type ParticipantsSummaryResponse =
  components['schemas']['ParticipantsSummaryResponse'];

/** Двойной модуль API-слоя participants (без 'use client'): чистые
 * fetch-функции и queryOptions-фабрики канона #887 — общий источник
 * ключ+fetch для клиентских хуков и серверного префетча. Хуки — в
 * hooks.ts ('use client'); хуки списка переиспользуют ParticipantsSummary
 * отсюда же. */

/** Чистый fetch счётчиков хаба — общее горло хука и серверного префетча
 * #887. */
export async function fetchParticipantsSummary(
  transport: ApiTransport = apiClient,
): Promise<ParticipantsSummary> {
  const response = await transport<ParticipantsSummaryResponse>(
    '/participants/summary',
  );
  return {
    participantsCount: response.participants_count,
    accessiblePropertiesCount: response.accessible_properties_count,
  };
}

/** Опции счётчиков хаба (канон #887): один источник ключ+fetch для хука
 * и серверного префетча. */
export function participantsSummaryQueryOptions(
  transport: ApiTransport = apiClient,
): UseQueryOptions<ParticipantsSummary, ApiError, ParticipantsSummary, typeof participantsKeys.summary> {
  return queryOptions({
    queryKey: participantsKeys.summary,
    queryFn: () => fetchParticipantsSummary(transport),
  });
}

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
