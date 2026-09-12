'use client';

import {
  keepPreviousData,
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
  type InfiniteData,
  type UseInfiniteQueryResult,
  type UseMutationResult,
  type UseQueryResult,
} from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import type { ApiError } from '@/shared/api/errors';
import { mapPropertyResponse } from '@/entities/property';
import type { Property } from '@/entities/property';
import { propertyKeys } from '@/shared/api/query-keys';
import type { components, operations } from '@/shared/api/dto';

type PropertyResponse = components['schemas']['PropertyResponse'];
type PropertiesResponse = components['schemas']['PropertiesResponse'];
type PropertiesSearchResponse = components['schemas']['PropertiesSearchResponse'];
type PropertyCreateRequest = components['schemas']['PropertyCreateRequest'];
type PropertyUpdateRequest = components['schemas']['PropertyUpdateRequest'];
type PropertyPhoto = components['schemas']['PropertyPhoto'];
type AddressSuggestionsResponse =
  components['schemas']['AddressSuggestionsResponse'];
type AddressSuggestion = components['schemas']['AddressSuggestion'];

export type DeletePropertyMode =
  operations['deleteProperty']['parameters']['query']['mode'];

/** Чистый fetch списка объектов — общее горло хука и прогрева хабов
 * #626 (кэш прогревается тем же кодом, что читает экран). */
export async function fetchProperties(): Promise<Property[]> {
  const response = await apiClient<PropertiesResponse>('/properties');
  return response.items.map(mapPropertyResponse);
}

export function useProperties(
  options: { enabled?: boolean } = {},
): UseQueryResult<Property[], ApiError> {
  return useQuery({
    queryKey: propertyKeys.list,
    queryFn: fetchProperties,
    enabled: options.enabled,
  });
}

export type PropertiesListResult = {
  readonly items: Property[];
  readonly hiddenSharedCount: number;
};

// Same /properties endpoint as useProperties, but also surfaces
// hidden_shared_count (shared objects hidden from the recipient due to a
// tariff slot shortage). Use this only where the count is needed — the rest
// of the app keeps useProperties for the plain list. Shares the
// propertyKeys.list prefix so mutations invalidate both queries together.
export function usePropertiesWithMeta(
  options: { enabled?: boolean } = {},
): UseQueryResult<PropertiesListResult, ApiError> {
  return useQuery({
    queryKey: [...propertyKeys.list, 'meta'],
    queryFn: async () => {
      const response = await apiClient<PropertiesResponse>('/properties');
      return {
        items: response.items.map(mapPropertyResponse),
        hiddenSharedCount: response.hidden_shared_count,
      };
    },
    enabled: options.enabled,
  });
}

export function useArchivedProperties(
  options: { enabled?: boolean } = {},
): UseQueryResult<Property[], ApiError> {
  return useQuery({
    queryKey: [...propertyKeys.list, 'archived'],
    queryFn: async () => {
      const response = await apiClient<PropertiesResponse>('/properties/archive');
      return response.items.map(mapPropertyResponse);
    },
    enabled: options.enabled,
  });
}

/** Порция поиска объектов: контракт #601 — порции по 50. */
export const PROPERTIES_SEARCH_PAGE_SIZE = 50;

/**
 * Порция поиска объектов (#601): строки плюс keyset-продолжение — opaque-
 * курсор следующей порции, null = совпадения исчерпаны.
 */
export type PropertiesSearchPageData = {
  readonly items: Property[];
  readonly nextCursor: string | null;
};

/** Общее горло порции GET /properties/search: search — обязательный
 * регистронезависимый фильтр по названию и адресу (клиентски тримится),
 * cursor — keyset-продолжение прошлого ответа, undefined читает с начала. */
async function fetchPropertiesSearchPage(params: {
  search: string;
  cursor?: string;
}): Promise<PropertiesSearchPageData> {
  const query = new URLSearchParams({ search: params.search });
  query.set('limit', String(PROPERTIES_SEARCH_PAGE_SIZE));
  if (params.cursor) {
    query.set('cursor', params.cursor);
  }
  const response = await apiClient<PropertiesSearchResponse>(
    `/properties/search?${query.toString()}`,
  );
  return {
    items: response.items.map(mapPropertyResponse),
    nextCursor: response.nextCursor ?? null,
  };
}

/**
 * Поиск объектов (глобальная страница «Объектов», тикет #601): порции по 50
 * keyset-курсором — pageParam это курсор прошлого ответа, смена queryKey
 * (новый запрос) начинает свежий обход с пустого курсора — sentinel не
 * наследует позицию прошлых порций. Склейка порций без дедупа: один
 * сортировочный ключ (название, id), повторы keyset не порождает —
 * канон книги контактов (#600). keepPreviousData — прежняя выдача
 * держится на экране, пока едет запрос с новым ?search= (канон платежей
 * #609); скелетон — только когда данных нет вовсе. Пустой (после трима)
 * запрос контракт не проходит (search обязателен) — чтение не
 * запускается, экран показывает подсказку.
 */
export function usePropertiesSearch(
  search: string,
): UseInfiniteQueryResult<Property[], ApiError> {
  return useInfiniteQuery({
    queryKey: propertyKeys.search(search),
    queryFn: ({ pageParam }) =>
      fetchPropertiesSearchPage({ search, cursor: pageParam }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) => lastPage.nextCursor ?? undefined,
    select: (data: InfiniteData<PropertiesSearchPageData>) =>
      data.pages.flatMap((page) => page.items),
    placeholderData: keepPreviousData,
    enabled: search.trim() !== '',
  });
}

export function useProperty(
  id: string,
): UseQueryResult<Property, ApiError> {
  return useQuery({
    queryKey: propertyKeys.detail(id),
    queryFn: async () => {
      const response = await apiClient<PropertyResponse>(`/properties/${id}`);
      return mapPropertyResponse(response);
    },
    enabled: Boolean(id),
  });
}

export function useCreateProperty(): UseMutationResult<
  Property,
  ApiError,
  PropertyCreateRequest
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (data: PropertyCreateRequest) => {
      const response = await apiClient<PropertyResponse>('/properties', {
        method: 'POST',
        body: JSON.stringify(data),
      });
      return mapPropertyResponse(response);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: propertyKeys.list });
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
      void queryClient.invalidateQueries({ queryKey: propertyKeys.list });
      void queryClient.invalidateQueries({ queryKey: propertyKeys.detail(id) });
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
      void queryClient.invalidateQueries({ queryKey: propertyKeys.list });
      void queryClient.invalidateQueries({ queryKey: propertyKeys.detail(id) });
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
      void queryClient.invalidateQueries({ queryKey: propertyKeys.list });
      void queryClient.invalidateQueries({ queryKey: propertyKeys.detail(id) });
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

export function useDeletePropertyPhoto(): UseMutationResult<
  void,
  ApiError,
  { propertyId: string; photoId: string }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ propertyId, photoId }) =>
      apiClient<void>(`/properties/${propertyId}/photos/${photoId}`, {
        method: 'DELETE',
      }),
    onSuccess: (_, { propertyId }) => {
      void queryClient.invalidateQueries({ queryKey: propertyKeys.list });
      void queryClient.invalidateQueries({ queryKey: propertyKeys.detail(propertyId) });
    },
  });
}

export function useDeleteProperty(): UseMutationResult<
  void,
  ApiError,
  { id: string; mode: DeletePropertyMode }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, mode }) =>
      apiClient<void>(`/properties/${id}?mode=${mode}`, {
        method: 'DELETE',
      }),
    onSuccess: (_, { id }) => {
      void queryClient.invalidateQueries({ queryKey: propertyKeys.list });
      queryClient.removeQueries({ queryKey: propertyKeys.detail(id) });
    },
  });
}
