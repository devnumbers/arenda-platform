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
import { propertyKeys } from './keys';
import type { components } from '@/shared/api/generated';

type PropertyResponse = components['schemas']['PropertyResponse'];
type PropertyCreateRequest = components['schemas']['PropertyCreateRequest'];
type PropertyUpdateRequest = components['schemas']['PropertyUpdateRequest'];

export function useProperties(): UseQueryResult<PropertyResponse[], ApiError> {
  return useQuery({
    queryKey: propertyKeys.all,
    queryFn: () => apiClient<PropertyResponse[]>('/properties'),
  });
}

export function useProperty(
  id: string,
): UseQueryResult<PropertyResponse, ApiError> {
  return useQuery({
    queryKey: propertyKeys.detail(id),
    queryFn: () => apiClient<PropertyResponse>(`/properties/${id}`),
    enabled: Boolean(id),
  });
}

export function useCreateProperty(): UseMutationResult<
  PropertyResponse,
  ApiError,
  PropertyCreateRequest
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: PropertyCreateRequest) =>
      apiClient<PropertyResponse>('/properties', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: propertyKeys.all });
    },
  });
}

export function useUpdateProperty(): UseMutationResult<
  PropertyResponse,
  ApiError,
  { id: string; data: PropertyUpdateRequest }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }) =>
      apiClient<PropertyResponse>(`/properties/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(data),
      }),
    onSuccess: (_, { id }) => {
      queryClient.invalidateQueries({ queryKey: propertyKeys.all });
      queryClient.invalidateQueries({ queryKey: propertyKeys.detail(id) });
    },
  });
}

export function useArchiveProperty(): UseMutationResult<
  PropertyResponse,
  ApiError,
  string
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id) =>
      apiClient<PropertyResponse>(`/properties/${id}/archive`, {
        method: 'POST',
      }),
    onSuccess: (_, id) => {
      queryClient.invalidateQueries({ queryKey: propertyKeys.all });
      queryClient.invalidateQueries({ queryKey: propertyKeys.detail(id) });
    },
  });
}

export function useUnarchiveProperty(): UseMutationResult<
  PropertyResponse,
  ApiError,
  string
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id) =>
      apiClient<PropertyResponse>(`/properties/${id}/unarchive`, {
        method: 'POST',
      }),
    onSuccess: (_, id) => {
      queryClient.invalidateQueries({ queryKey: propertyKeys.all });
      queryClient.invalidateQueries({ queryKey: propertyKeys.detail(id) });
    },
  });
}
