'use client';

import { useMutation, useQuery, useQueryClient, type UseMutationResult, type UseQueryResult } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import type { ApiError } from '@/shared/api/errors';
import type { components } from '@/shared/api/dto';
import { participantsKeys } from '@/shared/api/query-keys';
import { mapParticipant } from '@/entities/participants';
import type { Participant } from '@/entities/participants';

type ParticipantsSummaryResponse =
  components['schemas']['ParticipantsSummaryResponse'];
type ParticipantsResponse = components['schemas']['ParticipantsResponse'];

/** Счётчики хаба «Совместный доступ» (#693, GET /participants/summary):
 * участники читающего (агрегат members∪invitations его скоупа) и чужие
 * объекты, к которым у него есть активный доступ. */
export type ParticipantsSummary = {
  readonly participantsCount: number;
  readonly accessiblePropertiesCount: number;
};

/** Счётчики хаба «Совместный доступ» (карта #692, тикет #696). */
export function useParticipantsSummary(): UseQueryResult<ParticipantsSummary, ApiError> {
  return useQuery({
    queryKey: participantsKeys.summary,
    queryFn: async (): Promise<ParticipantsSummary> => {
      const response = await apiClient<ParticipantsSummaryResponse>(
        '/participants/summary',
      );
      return {
        participantsCount: response.participants_count,
        accessiblePropertiesCount: response.accessible_properties_count,
      };
    },
  });
}

/** Список «Ваши участники» (тикет #697, GET /participants #693): агрегаты
 * участников сцопа читающего, серверный порядок — имя ASC. Объём мал
 * (тарифные слоты) — целиком, без пагинации; поиск и сортировка
 * клиентские (entities/participants). */
export function useParticipantsList(): UseQueryResult<Participant[], ApiError> {
  return useQuery({
    queryKey: participantsKeys.list(),
    queryFn: async (): Promise<Participant[]> => {
      const response = await apiClient<ParticipantsResponse>('/participants');
      return response.items.map(mapParticipant);
    },
  });
}

/** Итог массового отзыва (#697): число удалённых и неудалённых — частичный
 * сбой пары DELETE не должен выглядеть полным успехом. */
export type RevokeAllParticipantsResult = {
  readonly revoked: number;
  readonly failed: number;
};

/** «Отозвать доступ всем» (#697): DELETE /participants/{id} (#694) по
 * каждому участнику текущего списка. Bulk-эндпоинта в контракте нет;
 * повторный DELETE по уже отозванному человеку даёт приватный 404 без
 * побочных эффектов, поэтому гонка параллельных запросов безопасна. Кэш
 * участников инвалидируется всегда (включая summary хаба) — и после
 * успеха, и при частичном сбое. */
export function useRevokeAllParticipants(): UseMutationResult<
  RevokeAllParticipantsResult,
  ApiError,
  readonly string[]
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (participantIds) => {
      const results = await Promise.allSettled(
        participantIds.map((id) =>
          apiClient<void>(`/participants/${encodeURIComponent(id)}`, { method: 'DELETE' }),
        ),
      );
      let revoked = 0;
      let failed = 0;
      for (const result of results) {
        if (result.status === 'fulfilled') {
          revoked += 1;
        } else {
          failed += 1;
        }
      }
      return { revoked, failed };
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: participantsKeys.all });
    },
  });
}
