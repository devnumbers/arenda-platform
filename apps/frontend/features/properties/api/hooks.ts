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
import type { IsoDate } from '@/shared/lib/calendar';
import { mapPropertyResponse } from '@/entities/property';
import type { Property } from '@/entities/property';
import { propertyKeys } from '@/shared/api/query-keys';
import { keysetNextPageParam } from '@/shared/lib/keyset';
import { resolvePropertiesLandingHref } from '../lib/property-landing';
import type { SharedAccessRole } from '@/shared/model/access';
import type { components } from '@/shared/api/dto';

type PropertyResponse = components['schemas']['PropertyResponse'];
type PropertiesResponse = components['schemas']['PropertiesResponse'];
type PropertiesSearchResponse = components['schemas']['PropertiesSearchResponse'];
type PropertyCreateRequest = components['schemas']['PropertyCreateRequest'];
type PropertyUpdateRequest = components['schemas']['PropertyUpdateRequest'];
type PropertyPhoto = components['schemas']['PropertyPhoto'];
type AddressSuggestionsResponse =
  components['schemas']['AddressSuggestionsResponse'];
type AddressSuggestion = components['schemas']['AddressSuggestion'];

/** Подвесший чужой объект списка (карта #692, тикет #702): блюр-карточка —
 * обычная карточка объекта (название, адрес — рендерятся под blur, Figma
 * 2213-99113) плюс контакт владельца для шита причины (Figma 2229-100002;
 * почта владельца — сознательная экспозиция этого экрана). */
export type SuspendedSharedProperty = {
  readonly propertyId: string;
  readonly accessRole: SharedAccessRole;
  readonly name: string;
  readonly address: string;
  readonly ownerName: string;
  readonly ownerEmail: string;
};

function mapSuspendedShared(
  dto: NonNullable<PropertiesResponse['suspended_shared']>,
): SuspendedSharedProperty[] {
  return dto.map((item) => ({
    propertyId: item.property_id,
    accessRole: item.access_role,
    name: item.name,
    address: item.address,
    ownerName: item.owner_name,
    ownerEmail: item.owner_email,
  }));
}

export type PropertiesListResult = {
  readonly items: Property[];
  /** Подвесшие общие объекты получателя (#702) — блюр-карточки хаба вместо
   * сноски hidden_shared_count (#158 T4). */
  readonly suspendedShared: SuspendedSharedProperty[];
  /** «Сегодня владельца» (ADR 0048) — граница бейджа «Осталось N месяцев» (#586). */
  readonly today: IsoDate;
};

/** Полный payload GET /properties — строки, suspended-плейсхолдеры (#702) и
 * «сегодня владельца» (ADR 0048). Общее горло обоих хуков и прогрева хабов
 * #626: один cache entry на propertyKeys.list, useProperties и
 * usePropertiesWithMeta — лишь проекции над ним (кэш прогревается тем же
 * кодом, что читает экран). */
export async function fetchProperties(): Promise<PropertiesListResult> {
  const response = await apiClient<PropertiesResponse>('/properties');
  return {
    items: response.items.map(mapPropertyResponse),
    suspendedShared: response.suspended_shared
      ? mapSuspendedShared(response.suspended_shared)
      : [],
    today: response.today,
  };
}

/** Проекция «только строки» над общим cache entry. Module-level, чтобы
 * ссылка select была стабильной — react-query кэширует её результат. */
function selectPropertiesItems(meta: PropertiesListResult): Property[] {
  return meta.items;
}

export function useProperties(
  options: { enabled?: boolean; staleTime?: number } = {},
): UseQueryResult<Property[], ApiError> {
  return useQuery({
    queryKey: propertyKeys.list,
    queryFn: fetchProperties,
    select: selectPropertiesItems,
    enabled: options.enabled,
    staleTime: options.staleTime,
  });
}

// Тот же cache entry, что у useProperties (один ключ — один запрос,
// react-query дедуплицирует параллельных наблюдателей). Поверхности
// hidden_shared_count и today нужны не везде — остальное приложение
// читает useProperties с проекцией на строки.
export function usePropertiesWithMeta(
  options: { enabled?: boolean } = {},
): UseQueryResult<PropertiesListResult, ApiError> {
  return useQuery({
    queryKey: propertyKeys.list,
    queryFn: fetchProperties,
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

/**
 * «Сделать основным / Убрать из основных» (деталь объекта #588): атомарный
 * PUT pin (канон favorite-toggle, #577) — не read-modify-write PATCH.
 * Инвалидит список и деталь — порядок карточек держит сервер.
 */
export function useSetPropertyPin(): UseMutationResult<
  PropertyResponse,
  ApiError,
  { id: string; pinned: boolean }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, pinned }) =>
      apiClient<PropertyResponse>(`/properties/${id}/pin`, {
        method: 'PUT',
        body: JSON.stringify({ pinned }),
      }),
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

export function useDeleteProperty(): UseMutationResult<void, ApiError, { id: string }> {
  const queryClient = useQueryClient();
  return useMutation({
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
 * Адрес, куда ведёт таб «Объекты» (лендинг таба, карта #583): основной
 * объект → его страница; основного нет, но активный один → его страница;
 * иначе список. Пока список не загружен — список (безопасный фолбэк).
 * Хромовые поверхности (ScreenLayout → TabBar/DesktopSidebar) держат
 * кэш тёплым с коротким staleTime, чтобы ссылка жила без шторма запросов.
 */
export function usePropertiesLandingHref(): string {
  const { data } = useProperties({ staleTime: 60_000 });
  return resolvePropertiesLandingHref(data);
}
