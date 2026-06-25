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
type PropertiesResponse = components['schemas']['PropertiesResponse'];
type PropertyCreateRequest = components['schemas']['PropertyCreateRequest'];
type PropertyUpdateRequest = components['schemas']['PropertyUpdateRequest'];
type PropertyPhoto = components['schemas']['PropertyPhoto'];
type AddressSuggestionsResponse =
  components['schemas']['AddressSuggestionsResponse'];
type AddressSuggestion = components['schemas']['AddressSuggestion'];

export function useProperties(): UseQueryResult<PropertyResponse[], ApiError> {
  return useQuery({
    queryKey: propertyKeys.list,
    queryFn: async () => {
      const response = await apiClient<PropertiesResponse>('/properties');
      return response.items;
    },
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
      queryClient.invalidateQueries({ queryKey: propertyKeys.list });
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
      queryClient.invalidateQueries({ queryKey: propertyKeys.list });
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
      queryClient.invalidateQueries({ queryKey: propertyKeys.list });
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
      queryClient.invalidateQueries({ queryKey: propertyKeys.list });
      queryClient.invalidateQueries({ queryKey: propertyKeys.detail(id) });
    },
  });
}

export function useAddressSuggestions(
  query: string,
): UseQueryResult<AddressSuggestion[], ApiError> {
  return useQuery({
    queryKey: propertyKeys.addressSuggestions(query),
    queryFn: async () => {
      const response = await apiClient<AddressSuggestionsResponse>(
        `/dadata/suggestions/address?query=${encodeURIComponent(query)}`,
      );
      return response.suggestions;
    },
    enabled: query.trim().length >= 3,
    staleTime: 30 * 1000,
  });
}

export function useUploadPropertyPhoto(): UseMutationResult<
  PropertyPhoto,
  ApiError,
  { propertyId: string; file: File }
> {
  return useMutation({
    mutationFn: ({ propertyId, file }) => {
      const formData = new FormData();
      formData.append('file', file);
      return apiClient<PropertyPhoto>(`/properties/${propertyId}/photos`, {
        method: 'POST',
        body: formData,
      });
    },
  });
}
