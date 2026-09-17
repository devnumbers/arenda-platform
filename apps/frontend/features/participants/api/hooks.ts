'use client';

import { useQuery, type UseQueryResult } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import type { ApiError } from '@/shared/api/errors';
import type { components } from '@/shared/api/dto';
import { participantsKeys } from '@/shared/api/query-keys';

type ParticipantsSummaryResponse =
  components['schemas']['ParticipantsSummaryResponse'];

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
