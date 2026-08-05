'use client';

import {
    useMutation,
    useQuery,
    useQueryClient,
    type UseMutationResult,
    type UseQueryResult,
} from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { ApiError } from '@/shared/api/errors';
import { mapPropertyAccessMemberResponse } from '@/entities/access/model/mappers';
import type { PropertyAccessMember } from '@/entities/access/model/types';
import type { components } from '@/shared/api/generated';
import { accessKeys } from './keys';

type PropertyAccessMembersResponse =
    components['schemas']['PropertyAccessMembersResponse'];
type PropertyAccessMemberResponse =
    components['schemas']['PropertyAccessMemberResponse'];
type PropertyAccessMemberCreateRequest =
    components['schemas']['PropertyAccessMemberCreateRequest'];
type PropertyAccessMemberUpdateRequest =
    components['schemas']['PropertyAccessMemberUpdateRequest'];

export type AddMemberInput = {
    readonly userId: string;
    readonly role: 'full_access' | 'viewer';
};

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

export function useCreatePropertyAccessMember(
    propertyId: string,
): UseMutationResult<PropertyAccessMember, ApiError, AddMemberInput> {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: async (data: AddMemberInput) => {
            const body: PropertyAccessMemberCreateRequest = {
                user_id: data.userId,
                role: data.role,
            };
            const response = await apiClient<PropertyAccessMemberResponse>(
                `/properties/${propertyId}/access/members`,
                { method: 'POST', body: JSON.stringify(body) },
            );
            return mapPropertyAccessMemberResponse(response);
        },
        onSuccess: () => {
            void queryClient.invalidateQueries({
                queryKey: accessKeys.list(propertyId),
            });
        },
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
        onSuccess: () => {
            void queryClient.invalidateQueries({
                queryKey: accessKeys.list(propertyId),
            });
        },
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
        onSuccess: () => {
            void queryClient.invalidateQueries({
                queryKey: accessKeys.list(propertyId),
            });
        },
    });
}

export function useLeaveProperty(
    propertyId: string,
): UseMutationResult<void, ApiError, void> {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: async () => {
            await apiClient<void>(
                `/properties/${propertyId}/access/members/self`,
                { method: 'DELETE' },
            );
        },
        onSuccess: () => {
            void queryClient.invalidateQueries({
                queryKey: accessKeys.list(propertyId),
            });
        },
    });
}
