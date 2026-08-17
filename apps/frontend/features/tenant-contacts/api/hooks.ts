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
import { mapTenantContactResponse } from '@/entities/tenant-contact';
import type { TenantContact } from '@/entities/tenant-contact';
import { tenantContactKeys } from '@/shared/api/query-keys';
import type { components } from '@/shared/api/dto';

type TenantContactResponse = components['schemas']['TenantContactResponse'];
type TenantContactCreateRequest =
  components['schemas']['TenantContactCreateRequest'];
type TenantContactUpdateRequest =
  components['schemas']['TenantContactUpdateRequest'];
type TenantContactsResponse = components['schemas']['TenantContactsResponse'];

export function useTenantContacts(): UseQueryResult<
  TenantContact[],
  ApiError
> {
  return useQuery({
    queryKey: tenantContactKeys.all,
    queryFn: async () => {
      const response = await apiClient<TenantContactsResponse>('/tenant-contacts');
      return response.items.map(mapTenantContactResponse);
    },
  });
}

export function useTenantContact(
  id: string,
): UseQueryResult<TenantContact, ApiError> {
  return useQuery({
    queryKey: tenantContactKeys.detail(id),
    queryFn: async () => {
      const response = await apiClient<TenantContactResponse>(`/tenant-contacts/${id}`);
      return mapTenantContactResponse(response);
    },
    enabled: Boolean(id),
  });
}

export function useCreateTenantContact(): UseMutationResult<
  TenantContact,
  ApiError,
  TenantContactCreateRequest
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (data: TenantContactCreateRequest) => {
      const response = await apiClient<TenantContactResponse>('/tenant-contacts', {
        method: 'POST',
        body: JSON.stringify(data),
      });
      return mapTenantContactResponse(response);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: tenantContactKeys.all });
    },
  });
}

export function useUpdateTenantContact(): UseMutationResult<
  TenantContact,
  ApiError,
  { id: string; data: TenantContactUpdateRequest }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, data }) => {
      const response = await apiClient<TenantContactResponse>(`/tenant-contacts/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(data),
      });
      return mapTenantContactResponse(response);
    },
    onSuccess: (_, { id }) => {
      queryClient.invalidateQueries({ queryKey: tenantContactKeys.all });
      queryClient.invalidateQueries({
        queryKey: tenantContactKeys.detail(id),
      });
    },
  });
}
