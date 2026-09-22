'use client';

import { useMutation, useQuery, useQueryClient, type UseMutationResult, type UseQueryResult } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import type { ApiError } from '@/shared/api/errors';
import type { components } from '@/shared/api/dto';
import { accessKeys, participantsKeys, propertyKeys } from '@/shared/api/query-keys';
import type { QueryClient } from '@tanstack/react-query';
import { mapParticipant } from '@/entities/participants';
import type { Participant } from '@/entities/participants';
import {
  countGrantOutcomes,
  toAddParticipantPropertiesWireRequest,
  toInviteParticipantWireRequest,
  type AddParticipantPropertiesCommand,
  type InviteParticipantCommand,
} from './wire';

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

/** Строки участников проецируются в двух видах: агрегаты
 * GET /participants* и списки объекта GET /properties/{id}/access/members
 * (#700). Инвалидация обеих семей обязательна у каждой participants-мутации:
 * окно staleTime (#626) иначе отдаёт спискам объекта устаревший кэш. */
function invalidateParticipantProjections(queryClient: QueryClient): void {
  void queryClient.invalidateQueries({ queryKey: participantsKeys.all });
  void queryClient.invalidateQueries({ queryKey: accessKeys.all });
}

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
 * каждому участнику текущего списка. Bulk-эндпоинта в контракте нет —
 * каркас партии общий с «Покинуть все объекты» (deleteBatchCounting). Кэш
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
      const { done, failed } = await deleteBatchCounting(
        participantIds.map((id) => `/participants/${encodeURIComponent(id)}`),
      );
      return { revoked: done, failed };
    },
    onSettled: () => invalidateParticipantProjections(queryClient),
  });
}

/** Страница участника (#698, GET /participants/{id} #693): агрегат одного
 * человека с per-object ногами. 404 (человек вне скоупа читающего или уже
 * отозван) доходит до экрана как ApiError со статусом — deep-link-политика
 * «Участник не найден»; 404 не ретрается глобальным предикатом (#769,
 * shared/providers/query-retry) — экран показывает состояние сразу. */
export function useParticipant(
  participantId: string,
): UseQueryResult<Participant, ApiError> {
  return useQuery({
    queryKey: participantsKeys.detail(participantId),
    queryFn: async (): Promise<Participant> => {
      const response = await apiClient<components['schemas']['ParticipantResponse']>(
        `/participants/${encodeURIComponent(participantId)}`,
      );
      return mapParticipant(response);
    },
    enabled: participantId.length > 0,
  });
}

/** «Отозвать и удалить» со страницы участника (#698, DELETE
 * /participants/{id} #694): снимает все ноги человека на объектах читающего.
 * Инвалидация всегда (onSettled, канон useRevokeAllParticipants) — и после
 * успеха, и при сбое: список/счётчики/агрегат перечитываются. */
export function useRevokeParticipant(): UseMutationResult<void, ApiError, string> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (participantId: string) => {
      await apiClient<void>(`/participants/${encodeURIComponent(participantId)}`, {
        method: 'DELETE',
      });
    },
    onSettled: () => invalidateParticipantProjections(queryClient),
  });
}

/** Итог пакетного DELETE: число исполненных и неудавшихся — частичный
 * сбой партии не должен выглядеть полным успехом (общий каркас «Отозвать
 * всех» #697 и «Покинуть все объекты» #701). */
export type BatchDeleteResult = {
  readonly done: number;
  readonly failed: number;
};

/** Подсчёт исходов партии параллельных DELETE: повторный DELETE даёт
 * приватный 404 без побочных эффектов, поэтому гонка параллельных
 * запросов безопасна. */
async function deleteBatchCounting(
  urls: ReadonlyArray<string>,
): Promise<BatchDeleteResult> {
  const results = await Promise.allSettled(
    urls.map((url) => apiClient<void>(url, { method: 'DELETE' })),
  );
  let done = 0;
  let failed = 0;
  for (const result of results) {
    if (result.status === 'fulfilled') {
      done += 1;
    } else {
      failed += 1;
    }
  }
  return { done, failed };
}

/** Инвалидация самовыхода (#701): проекции участников (summary хаба
 * считает чужие объекты) плюс сам список объектов с деталями — ряд уходит
 * со экрана, детали чужого объекта перестают быть доступны. */
function invalidateLeaveProjections(queryClient: QueryClient): void {
  invalidateParticipantProjections(queryClient);
  void queryClient.invalidateQueries({ queryKey: propertyKeys.all });
}

/** «Покинуть объект» со экрана «Объекты пользователей» (карта #692, тикет
 * #701; DELETE /properties/{id}/access/members/self): самовыход участника.
 * Инвалидация в onSettled — канон participants-мутаций. */
export function useLeaveProperty(): UseMutationResult<void, ApiError, string> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (propertyId: string) => {
      await apiClient<void>(
        `/properties/${encodeURIComponent(propertyId)}/access/members/self`,
        { method: 'DELETE' },
      );
    },
    onSettled: () => invalidateLeaveProjections(queryClient),
  });
}

/** «Покинуть все объекты» (#701, кебаб шапки): DELETE members/self по
 * каждому чужому объекту текущего списка. Bulk-эндпоинта в контракте нет;
 * повторный DELETE безопасен (см. deleteBatchCounting). Кэш инвалидируется
 * всегда (onSettled, канон useRevokeAllParticipants) — список под попапом
 * уже пуст. */
export function useLeaveAllProperties(): UseMutationResult<
  BatchDeleteResult,
  ApiError,
  readonly string[]
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (propertyIds) =>
      deleteBatchCounting(
        propertyIds.map(
          (id) => `/properties/${encodeURIComponent(id)}/access/members/self`,
        ),
      ),
    onSettled: () => invalidateLeaveProjections(queryClient),
  });
}

/** Исход «Пригласить в объект» (#698, POST /participants/{id}/properties
 * #694): per-object исходы партии — успехи и skipped_* (уже выдан,
 * архивный, чужой и т.д.). */
export type AddParticipantPropertiesResult = {
  readonly granted: number;
  readonly skipped: number;
};

/** «Пригласить в объект» (#698): одна роль на выбранные объекты
 * (AddParticipantPropertiesCommand → wire, api/wire). Счёт granted/skipped
 * считает экран — партия может смешать исходы (ParticipantGrantOutcome
 * #694); частичный исход макетом не различён — попап один (решение
 * владельца на приёмке). Инвалидация в onSettled: переход на страницу
 * участника происходит в onSuccess, агрегат перечитается там. */
export function useAddParticipantProperties(
  participantId: string,
): UseMutationResult<
  AddParticipantPropertiesResult,
  ApiError,
  AddParticipantPropertiesCommand
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (command) => {
      const response = await apiClient<components['schemas']['ParticipantGrantResultsResponse']>(
        `/participants/${encodeURIComponent(participantId)}/properties`,
        {
          method: 'POST',
          body: JSON.stringify(toAddParticipantPropertiesWireRequest(command)),
        },
      );
      return countGrantOutcomes(response.items);
    },
    onSettled: () => invalidateParticipantProjections(queryClient),
  });
}

/** Исход приглашения участника (#699, POST /participants/invite #694):
 * per-object исходы партии — успехи и skipped_* (тот же ответ, что у
 * «Пригласить в объект» #698). */
export type InviteParticipantResult = {
  readonly granted: number;
  readonly skipped: number;
};

/** Приглашение участника из хаба (#699): одна почта и роль на снапшот
 * выбранных объектов. granted = активные + suspended (слоты получателя) +
 * pending (письмо) — человек приглашён в любом из этих исходов; когда
 * granted=0 при выбранных объектах, все ушли в skipped_* — экран
 * показывает ошибку и остаётся на месте. Инвалидация в onSettled — канон
 * useAddParticipantProperties: успех уводит назад по истории, агрегаты и
 * счётчики перечитает целевой экран. */
export function useInviteParticipant(): UseMutationResult<
  InviteParticipantResult,
  ApiError,
  InviteParticipantCommand
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (command) => {
      const response = await apiClient<components['schemas']['ParticipantGrantResultsResponse']>(
        '/participants/invite',
        {
          method: 'POST',
          body: JSON.stringify(toInviteParticipantWireRequest(command)),
        },
      );
      return countGrantOutcomes(response.items);
    },
    onSettled: () => invalidateParticipantProjections(queryClient),
  });
}
