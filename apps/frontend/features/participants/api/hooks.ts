'use client';

import { useQuery, useQueryClient, type UseMutationResult, type UseQueryResult } from '@tanstack/react-query';
import { useGuardedMutation } from '@/shared/lib/hooks/use-guarded-mutation';
import { apiClient } from '@/shared/api/client';
import type { ApiError } from '@/shared/api/errors';
import type { components } from '@/shared/api/dto';
import { accessKeys, participantsKeys, propertyKeys } from '@/shared/api/query-keys';
import type { QueryClient } from '@tanstack/react-query';
import type { Participant } from '@/entities/participants';
import {
  countGrantOutcomes,
  toAddParticipantPropertiesWireRequest,
  toInviteParticipantWireRequest,
  type AddParticipantPropertiesCommand,
  type InviteParticipantCommand,
} from './wire';

import { participantQueryOptions, participantsListQueryOptions } from './queries';

/** Строки участников проецируются в двух видах: агрегаты
 * GET /participants* и списки объекта GET /properties/{id}/access/members
 * (#700). Инвалидация обеих семей обязательна у каждой participants-мутации:
 * окно staleTime (#626) иначе отдаёт спискам объекта устаревший кэш. */
function invalidateParticipantProjections(queryClient: QueryClient): void {
  void queryClient.invalidateQueries({ queryKey: participantsKeys.all });
  void queryClient.invalidateQueries({ queryKey: accessKeys.all });
}

/** Список «Участники» (экран #697, GET /participants #693): агрегаты
 * участников сцопа читающего, серверный порядок — имя ASC. Объём мал
 * (тарифные слоты) — целиком, без пагинации; поиск и сортировка
 * клиентские (entities/participants). `enabled: false` — чтение за
 * тарифным гейтом (402 на basic): раздел на basic рендерит апселл, не
 * спрашивая список (#1313). */
export function useParticipantsList(
  options: { readonly enabled?: boolean } = {},
): UseQueryResult<Participant[], ApiError> {
  return useQuery({
    ...participantsListQueryOptions(),
    enabled: options.enabled,
  });
}

/** «Отозвать доступ всем» (кебаб раздела #1313): глобальный
 * DELETE /participants (#1310) — одним вызовом снимает все членства и
 * отменяет все pending-приглашения по всем объектам читающего,
 * идемпотентен. Кэш участников инвалидируется всегда (onSettled) — и
 * после успеха, и при сбое. */
export function useRevokeAllParticipants(): UseMutationResult<void, ApiError, void> {
  const queryClient = useQueryClient();
  return useGuardedMutation({
    mutationFn: async () => {
      await apiClient<void>('/participants', { method: 'DELETE' });
    },
    onSettled: () => invalidateParticipantProjections(queryClient),
  });
}

/** Страница участника (#698, GET /participants/{id} #693): агрегат одного
 * человека с per-object ногами. 404 (человек вне скоупа читающего или уже
 * отозван) доходит до экрана как ApiError со статусом — deep-link-политика
 * «Участник не найден»; 404 не ретрается глобальным предикатом (#769,
 * shared/providers/query-retry) — экран показывает состояние сразу.
 * enabled — выключатель вызова (прецедент useHistoryHasEntries): экран
 * без участничьей потребности не запрашивает агрегат (лента истории
 * кормит им только шит «Объектов», карта #1582). */
export function useParticipant(
  participantId: string,
  options: { readonly enabled?: boolean } = {},
): UseQueryResult<Participant, ApiError> {
  return useQuery({
    ...participantQueryOptions({ participantId }),
    enabled: (options.enabled ?? true) && participantId.length > 0,
  });
}

/** «Отозвать и удалить» со страницы участника (#698, DELETE
 * /participants/{id} #694): снимает все ноги человека на объектах читающего.
 * Инвалидация всегда (onSettled, канон useRevokeAllParticipants) — и после
 * успеха, и при сбое: список/агрегат перечитываются. */
export function useRevokeParticipant(): UseMutationResult<void, ApiError, string> {
  const queryClient = useQueryClient();
  return useGuardedMutation({
    mutationFn: async (participantId: string) => {
      await apiClient<void>(`/participants/${encodeURIComponent(participantId)}`, {
        method: 'DELETE',
      });
    },
    onSettled: () => invalidateParticipantProjections(queryClient),
  });
}

/** Инвалидация самовыхода (#701): проекции участников плюс сам список
 * объектов с деталями — ряд уходит со экрана, детали чужого объекта
 * перестают быть доступны. */
function invalidateLeaveProjections(queryClient: QueryClient): void {
  invalidateParticipantProjections(queryClient);
  void queryClient.invalidateQueries({ queryKey: propertyKeys.all });
}

/** «Покинуть объект» (карта #692, тикет #701; DELETE
 * /properties/{id}/access/members/self): самовыход участника. Живёт на
 * «Участниках объекта» (D6 #1312), странице объекта и заглушке
 * suspended-объекта. Инвалидация в onSettled — канон participants-мутаций. */
export function useLeaveProperty(): UseMutationResult<void, ApiError, string> {
  const queryClient = useQueryClient();
  return useGuardedMutation({
    mutationFn: async (propertyId: string) => {
      await apiClient<void>(
        `/properties/${encodeURIComponent(propertyId)}/access/members/self`,
        { method: 'DELETE' },
      );
    },
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
  return useGuardedMutation({
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

/** Приглашение участника из раздела «Участники» (#699): одна почта и роль
 * на снапшот выбранных объектов. granted = активные + suspended (слоты
 * получателя) + pending (письмо) — человек приглашён в любом из этих
 * исходов; когда granted=0 при выбранных объектах, все ушли в skipped_* —
 * экран показывает ошибку и остаётся на месте. Инвалидация в onSettled —
 * канон useAddParticipantProperties: успех уводит назад по истории,
 * агрегаты перечитает целевой экран. */
export function useInviteParticipant(): UseMutationResult<
  InviteParticipantResult,
  ApiError,
  InviteParticipantCommand
> {
  const queryClient = useQueryClient();
  return useGuardedMutation({
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
