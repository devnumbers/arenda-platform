'use client';

import {
  keepPreviousData,
  useInfiniteQuery,
  useQuery,
  useQueryClient,
  type InfiniteData,
  type UseInfiniteQueryResult,
  type UseMutationResult,
  type UseQueryResult,
} from '@tanstack/react-query';
import { useGuardedMutation } from '@/shared/lib/hooks/use-guarded-mutation';
import { apiClient } from '@/shared/api/client';
import type { ApiError } from '@/shared/api/errors';
import { mapPropertyPhoto, mapPropertyResponse } from '@/entities/property';
import type { Property, PropertyPhoto } from '@/entities/property';
import { propertyKeys } from '@/shared/api/query-keys';
import { keysetNextPageParam, type KeysetPage } from '@/shared/lib/keyset';
import { resolvePropertiesNavItem, type PropertiesNavItem } from '../lib/properties-nav-item';
import type { components } from '@/shared/api/dto';

type PropertyResponse = components['schemas']['PropertyResponse'];
type PropertiesResponse = components['schemas']['PropertiesResponse'];

import {
  propertiesListQueryOptions,
  propertyDetailQueryOptions,
  type PropertiesListResult,
} from './queries';

export type {
  PropertiesListResult,
  PropertiesListQueryConfig,
  SuspendedSharedProperty,
} from './queries';
type PropertiesSearchResponse = components['schemas']['PropertiesSearchResponse'];
type PropertyCreateRequest = components['schemas']['PropertyCreateRequest'];
type PropertyUpdateRequest = components['schemas']['PropertyUpdateRequest'];
type PropertyPhotoDto = components['schemas']['PropertyPhoto'];
type AddressSuggestionsResponse =
  components['schemas']['AddressSuggestionsResponse'];
type AddressSuggestion = components['schemas']['AddressSuggestion'];

/** Проекция «только строки» над общим cache entry. Module-level, чтобы
 * ссылка select была стабильной — react-query кэширует её результат. */
function selectPropertiesItems(meta: PropertiesListResult): Property[] {
  return meta.items;
}

export function useProperties(
  options: { enabled?: boolean; staleTime?: number } = {},
): UseQueryResult<Property[], ApiError> {
  return useQuery({
    ...propertiesListQueryOptions(),
    select: selectPropertiesItems,
    enabled: options.enabled,
    staleTime: options.staleTime,
  });
}

// Тот же cache entry, что у useProperties (один ключ — один запрос,
// react-query дедуплицирует параллельных наблюдателей). Поверхности
// suspended_shared и today нужны не везде — остальное приложение
// читает useProperties с проекцией на строки.
export function usePropertiesWithMeta(
  options: { enabled?: boolean } = {},
): UseQueryResult<PropertiesListResult, ApiError> {
  return useQuery({
    ...propertiesListQueryOptions(),
    enabled: options.enabled,
  });
}

export function useArchivedProperties(
  options: { enabled?: boolean; staleTime?: number } = {},
): UseQueryResult<Property[], ApiError> {
  return useQuery({
    queryKey: [...propertyKeys.list, 'archived'],
    queryFn: async () => {
      const response = await apiClient<PropertiesResponse>('/properties/archive');
      return response.items.map(mapPropertyResponse);
    },
    enabled: options.enabled,
    staleTime: options.staleTime,
  });
}

/** Порция поиска объектов: контракт #601 — порции по 50. */
export const PROPERTIES_SEARCH_PAGE_SIZE = 50;

/**
 * Порция поиска объектов (#601): строки плюс keyset-продолжение — opaque-
 * курсор следующей порции, null = совпадения исчерпаны.
 */
export type PropertiesSearchPageData = KeysetPage<Property>;

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
    getNextPageParam: keysetNextPageParam,
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
    ...propertyDetailQueryOptions({ id }),
    enabled: Boolean(id),
  });
}

export function useCreateProperty(): UseMutationResult<
  Property,
  ApiError,
  PropertyCreateRequest
> {
  const queryClient = useQueryClient();
  return useGuardedMutation({
    mutationFn: async (data: PropertyCreateRequest) => {
      const response = await apiClient<PropertyResponse>('/properties', {
        method: 'POST',
        // Идемпотентный ключ (Т3 #1121): один ключ на логическую попытку
        // создания; повтор с ним вернёт первый результат, не второй объект.
        headers: { 'Idempotency-Key': crypto.randomUUID() },
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
  Property,
  ApiError,
  { id: string; data: PropertyUpdateRequest }
> {
  const queryClient = useQueryClient();
  return useGuardedMutation({
    mutationFn: async ({ id, data }) =>
      mapPropertyResponse(
        await apiClient<PropertyResponse>(`/properties/${id}`, {
          method: 'PATCH',
          body: JSON.stringify(data),
        }),
      ),
    onSuccess: (_, { id }) => {
      void queryClient.invalidateQueries({ queryKey: propertyKeys.list });
      void queryClient.invalidateQueries({ queryKey: propertyKeys.detail(id) });
    },
  });
}

export function useArchiveProperty(): UseMutationResult<
  Property,
  ApiError,
  string
> {
  const queryClient = useQueryClient();
  return useGuardedMutation({
    mutationFn: async (id) =>
      mapPropertyResponse(
        await apiClient<PropertyResponse>(`/properties/${id}/archive`, {
          method: 'POST',
        }),
      ),
    onSuccess: (_, id) => {
      void queryClient.invalidateQueries({ queryKey: propertyKeys.list });
      void queryClient.invalidateQueries({ queryKey: propertyKeys.detail(id) });
    },
  });
}

export function useUnarchiveProperty(): UseMutationResult<
  Property,
  ApiError,
  string
> {
  const queryClient = useQueryClient();
  return useGuardedMutation({
    mutationFn: async (id) =>
      mapPropertyResponse(
        await apiClient<PropertyResponse>(`/properties/${id}/unarchive`, {
          method: 'POST',
        }),
      ),
    onSuccess: (_, id) => {
      void queryClient.invalidateQueries({ queryKey: propertyKeys.list });
      void queryClient.invalidateQueries({ queryKey: propertyKeys.detail(id) });
    },
  });
}

/**
 * «Сделать основным / Убрать из основных» (деталь объекта #588): атомарный
 * PUT pin (канон favorite-toggle, #577) — не read-modify-write PATCH.
 * Инвалидит список и деталь — порядок карточек держит сервер.
 */
export function useSetPropertyPin(): UseMutationResult<
  Property,
  ApiError,
  { id: string; pinned: boolean }
> {
  const queryClient = useQueryClient();
  return useGuardedMutation({
    mutationFn: async ({ id, pinned }) =>
      mapPropertyResponse(
        await apiClient<PropertyResponse>(`/properties/${id}/pin`, {
          method: 'PUT',
          body: JSON.stringify({ pinned }),
        }),
      ),
    onSuccess: (_, { id }) => {
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
  return useGuardedMutation({
    mutationFn: async ({ propertyId, file }) => {
      const formData = new FormData();
      formData.append('file', file);
      return mapPropertyPhoto(
        await apiClient<PropertyPhotoDto>(`/properties/${propertyId}/photos`, {
          method: 'POST',
          body: formData,
        }),
      );
    },
  });
}

export function useDeletePropertyPhoto(): UseMutationResult<
  void,
  ApiError,
  { propertyId: string; photoId: string }
> {
  const queryClient = useQueryClient();
  return useGuardedMutation({
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

export function useDeleteProperty(): UseMutationResult<void, ApiError, { id: string }> {
  const queryClient = useQueryClient();
  return useGuardedMutation({
    // Deletion is total (ADR 0049): the property and all its data go together.
    mutationFn: ({ id }) =>
      apiClient<void>(`/properties/${id}`, {
        method: 'DELETE',
      }),
    onSuccess: (_, { id }) => {
      void queryClient.invalidateQueries({ queryKey: propertyKeys.list });
      queryClient.removeQueries({ queryKey: propertyKeys.detail(id) });
    },
  });
}

/**
 * Пункт «Объекты» единого хрома — подпись и адрес (карта #984): у базового
 * тарифа с ровно одним живым своим объектом и пустым архивом это «Объект»
 * со ссылкой на его страницу, иначе всегда «Объекты» на список. Тариф
 * приходит снаружи (ScreenLayout читает useMe — фича не может тянуть auth).
 * Архив дозапрашивается только базовому (правило считает и его), платным
 * тарифам он не нужен. Хромовые поверхности (ScreenLayout → TabBar/
 * DesktopSidebar) держат кэш тёплым с коротким staleTime, чтобы пункт
 * жил без шторма запросов; пока данные не загружены — «Объекты» на список.
 */
export function usePropertiesNavItem(
  tariffName: string | null | undefined,
): PropertiesNavItem {
  const { data } = useProperties({ staleTime: 60_000 });
  const { data: archived } = useArchivedProperties({
    enabled: tariffName === 'basic',
    staleTime: 60_000,
  });
  return resolvePropertiesNavItem(tariffName, data, archived);
}
