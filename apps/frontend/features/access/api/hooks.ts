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
type PropertyAccessInvitationCreateRequest =
    components['schemas']['PropertyAccessInvitationCreateRequest'];
type PropertyAccessMemberUpdateRequest =
    components['schemas']['PropertyAccessMemberUpdateRequest'];

export type InviteMemberInput = {
    readonly email: string;
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

export function useInvitePropertyAccessMember(
    propertyId: string,
): UseMutationResult<PropertyAccessMember, ApiError, InviteMemberInput> {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: async (data: InviteMemberInput) => {
            const body: PropertyAccessInvitationCreateRequest = {
                email: data.email,
                role: data.role,
            };
            const response = await apiClient<PropertyAccessMemberResponse>(
                `/properties/${propertyId}/access/invitations`,
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
        onSuccess: () => {
            void queryClient.invalidateQueries({
                queryKey: accessKeys.list(propertyId),
            });
        },
    });
}

export function useResendPropertyAccessInvitation(
    propertyId: string,
): UseMutationResult<void, ApiError, string> {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: async (invitationId: string) => {
            await apiClient<void>(
                `/properties/${propertyId}/access/invitations/${invitationId}/resend`,
                { method: 'POST' },
            );
        },
        onSuccess: () => {
            void queryClient.invalidateQueries({
                queryKey: accessKeys.list(propertyId),
            });
        },
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
