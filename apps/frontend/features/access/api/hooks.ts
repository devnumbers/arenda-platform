'use client';

import {
    useMutation,
    useQuery,
    useQueryClient,
    type QueryClient,
    type UseMutationResult,
    type UseQueryResult,
} from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import type { ApiError } from '@/shared/api/errors';
import { mapPropertyAccessMemberResponse } from '@/entities/access';
import type { AccessMemberStatus } from '@/entities/access';
import type { PropertyAccessMember } from '@/entities/access';
import type { components } from '@/shared/api/dto';
import { accessKeys, participantsKeys } from '@/shared/api/query-keys';

type PropertyAccessMembersResponse =
    components['schemas']['PropertyAccessMembersResponse'];
type PropertyAccessMemberResponse =
    components['schemas']['PropertyAccessMemberResponse'];
type PropertyAccessMemberUpdateRequest =
    components['schemas']['PropertyAccessMemberUpdateRequest'];

/** Строки участников проецируются в двух видах: списки объекта
 * GET /properties/{id}/access/members и агрегаты /participants*
 * (канон invalidateParticipantProjections, features/participants/api/hooks.ts).
 * Инвалидация обеих семей обязательна у каждой access-мутации — окно
 * staleTime (#626) иначе отдаёт агрегатам устаревший кэш; всегда
 * (onSettled) — перечитываются и после частичного сбоя. */
function invalidateAccessProjections(
    queryClient: QueryClient,
    propertyId: string,
): void {
    void queryClient.invalidateQueries({ queryKey: accessKeys.list(propertyId) });
    void queryClient.invalidateQueries({ queryKey: participantsKeys.all });
}

export type ChangeMemberRoleInput = {
    readonly role: 'full_access' | 'viewer';
};

export function usePropertyAccessMembers(
    propertyId: string,
): UseQueryResult<PropertyAccessMember[], ApiError> {
    return useQuery({
        queryKey: accessKeys.list(propertyId),
        queryFn: async () => {
            const response = await apiClient<PropertyAccessMembersResponse>(
                `/properties/${propertyId}/access/members`,
            );
            return response.items.map(mapPropertyAccessMemberResponse);
        },
        enabled: Boolean(propertyId),
    });
}

export function useUpdatePropertyAccessInvitation(
    propertyId: string,
): UseMutationResult<
    PropertyAccessMember,
    ApiError,
    { invitationId: string } & ChangeMemberRoleInput
> {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: async (data) => {
            const body: PropertyAccessMemberUpdateRequest = { role: data.role };
            const response = await apiClient<PropertyAccessMemberResponse>(
                `/properties/${propertyId}/access/invitations/${data.invitationId}`,
                { method: 'PATCH', body: JSON.stringify(body) },
            );
            return mapPropertyAccessMemberResponse(response);
        },
        onSettled: () => invalidateAccessProjections(queryClient, propertyId),
    });
}

export function useCancelPropertyAccessInvitation(
    propertyId: string,
): UseMutationResult<void, ApiError, string> {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: async (invitationId: string) => {
            await apiClient<void>(
                `/properties/${propertyId}/access/invitations/${invitationId}`,
                { method: 'DELETE' },
            );
        },
        onSettled: () => invalidateAccessProjections(queryClient, propertyId),
    });
}

export function useUpdatePropertyAccessMember(
    propertyId: string,
): UseMutationResult<
    PropertyAccessMember,
    ApiError,
    { memberId: string } & ChangeMemberRoleInput
> {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: async (data) => {
            const body: PropertyAccessMemberUpdateRequest = { role: data.role };
            const response = await apiClient<PropertyAccessMemberResponse>(
                `/properties/${propertyId}/access/members/${data.memberId}`,
                { method: 'PATCH', body: JSON.stringify(body) },
            );
            return mapPropertyAccessMemberResponse(response);
        },
        onSettled: () => invalidateAccessProjections(queryClient, propertyId),
    });
}

export function useDeletePropertyAccessMember(
    propertyId: string,
): UseMutationResult<void, ApiError, string> {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: async (memberId: string) => {
            await apiClient<void>(
                `/properties/${propertyId}/access/members/${memberId}`,
                { method: 'DELETE' },
            );
        },
        onSettled: () => invalidateAccessProjections(queryClient, propertyId),
    });
}

/** Итог «Отозвать доступ всем» на объекте (#700): частичный сбой партии
 * не должен выглядеть полным успехом — канон useRevokeAllParticipants. */
export type RevokeAllPropertyAccessMembersResult = {
    readonly revoked: number;
    readonly failed: number;
};

/** Строка списка участников для партийного отзыва: id и жизненный цикл
 * (active/suspended → DELETE members, pending → DELETE invitations). */
export type PropertyAccessMemberRef = {
    readonly id: string;
    readonly status: AccessMemberStatus;
};

/** «Отозвать доступ всем» на одном объекте (#700, макет 2035-82619):
 * DELETE /properties/{id}/access/members|invitations по каждому участнику
 * ряда — скоуп строго объектный (не /participants/{id}, который снял бы
 * ноги на всех объектах владельца). Bulk-эндпоинта нет; повторный DELETE
 * по уже отозванной строке даёт безопасную ошибку, гонка параллельных
 * запросов не страшна. Инвалидация всегда (onSettled), канон
 * invalidateAccessProjections: и список объекта, и агрегаты участников
 * перечитываются после частичного сбоя тоже. */
export function useRevokeAllPropertyAccessMembers(
    propertyId: string,
): UseMutationResult<
    RevokeAllPropertyAccessMembersResult,
    ApiError,
    ReadonlyArray<PropertyAccessMemberRef>
> {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: async (members) => {
            const results = await Promise.allSettled(
                members.map((member) =>
                    apiClient<void>(
                        member.status === 'pending'
                            ? `/properties/${propertyId}/access/invitations/${member.id}`
                            : `/properties/${propertyId}/access/members/${member.id}`,
                        { method: 'DELETE' },
                    ),
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
        onSettled: () => invalidateAccessProjections(queryClient, propertyId),
    });
}
