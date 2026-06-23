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
import { tenantContactKeys } from './keys';
import type { components } from '@/shared/api/generated';

type TenantContactResponse = components['schemas']['TenantContactResponse'];
type TenantContactCreateRequest =
  components['schemas']['TenantContactCreateRequest'];
type TenantContactUpdateRequest =
  components['schemas']['TenantContactUpdateRequest'];
type TenantContactsResponse = components['schemas']['TenantContactsResponse'];

export function useTenantContacts(): UseQueryResult<
  TenantContactsResponse,
  ApiError
> {
  return useQuery({
    queryKey: tenantContactKeys.all,
    queryFn: () => apiClient<TenantContactsResponse>('/tenant-contacts'),
  });
}

export function useTenantContact(
  id: string,
): UseQueryResult<TenantContactResponse, ApiError> {
  return useQuery({
    queryKey: tenantContactKeys.detail(id),
    queryFn: () => apiClient<TenantContactResponse>(`/tenant-contacts/${id}`),
    enabled: Boolean(id),
  });
}

export function useCreateTenantContact(): UseMutationResult<
  TenantContactResponse,
  ApiError,
  TenantContactCreateRequest
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: TenantContactCreateRequest) =>
      apiClient<TenantContactResponse>('/tenant-contacts', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: tenantContactKeys.all });
    },
  });
}

export function useUpdateTenantContact(): UseMutationResult<
  TenantContactResponse,
  ApiError,
  { id: string; data: TenantContactUpdateRequest }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }) =>
      apiClient<TenantContactResponse>(`/tenant-contacts/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(data),
      }),
    onSuccess: (_, { id }) => {
      queryClient.invalidateQueries({ queryKey: tenantContactKeys.all });
      queryClient.invalidateQueries({
        queryKey: tenantContactKeys.detail(id),
      });
    },
  });
}
