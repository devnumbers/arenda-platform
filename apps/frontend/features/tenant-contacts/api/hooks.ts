'use client';

import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import type { ApiError } from '@/shared/api/errors';
import { mapTenantContactResponse } from '@/entities/tenant-contact';
import type {
  TenantContact,
  TenantContactCreateRequest,
  TenantContactUpdateRequest,
} from '@/entities/tenant-contact';
import { tenantContactKeys } from '@/shared/api/query-keys';
import type { components } from '@/shared/api/dto';

type TenantContactResponse = components['schemas']['TenantContactResponse'];
type TenantContactCreateWireRequest =
  components['schemas']['TenantContactCreateRequest'];
type TenantContactUpdateWireRequest =
  components['schemas']['TenantContactUpdateRequest'];
type TenantContactsResponse = components['schemas']['TenantContactsResponse'];

// Команды приходят из виджетов в camelCase; wire-формат (snake_case) живёт
// только внутри этого модуля.
export function toCreateWireRequest(
  data: TenantContactCreateRequest,
): TenantContactCreateWireRequest {
  return {
    name: data.name,
    surname: data.surname,
    patronymic: data.patronymic,
    phone: data.phone,
    email: data.email,
    comment: data.comment,
    property_id: data.propertyId,
  };
}

export function toUpdateWireRequest(
  data: TenantContactUpdateRequest,
): TenantContactUpdateWireRequest {
  return {
    name: data.name,
    surname: data.surname,
    patronymic: data.patronymic,
    phone: data.phone,
    email: data.email,
    comment: data.comment,
  };
}

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
        body: JSON.stringify(toCreateWireRequest(data)),
      });
      return mapTenantContactResponse(response);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: tenantContactKeys.all });
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
        body: JSON.stringify(toUpdateWireRequest(data)),
      });
      return mapTenantContactResponse(response);
    },
    onSuccess: (_, { id }) => {
      void queryClient.invalidateQueries({ queryKey: tenantContactKeys.all });
      void queryClient.invalidateQueries({
        queryKey: tenantContactKeys.detail(id),
      });
    },
  });
}
