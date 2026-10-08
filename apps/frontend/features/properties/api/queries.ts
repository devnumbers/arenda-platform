import { queryOptions, type UseQueryOptions } from '@tanstack/react-query';
import { apiClient, type ApiTransport } from '@/shared/api/client';
import type { ApiError } from '@/shared/api/errors';
import type { components } from '@/shared/api/dto';
import { propertyKeys } from '@/shared/api/query-keys';
import type { IsoDate } from '@/shared/lib/calendar';
import type { Property } from '@/entities/property';
import { mapPropertyResponse } from '@/entities/property';
import type { SharedAccessRole } from '@/shared/model/access';

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

type PropertiesResponse = components['schemas']['PropertiesResponse'];
type PropertyResponse = components['schemas']['PropertyResponse'];

/** Двойной модуль API-слоя properties (без 'use client'): чистые
 * fetch-функции и queryOptions-фабрики канона #887 — общий источник
 * ключ+fetch для клиентских хуков, прогрева хабов #626 и серверного
 * префетча. Хуки — в hooks.ts ('use client'). */

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
export async function fetchProperties(
  transport: ApiTransport = apiClient,
): Promise<PropertiesListResult> {
  const response = await transport<PropertiesResponse>('/properties');
  return {
    items: response.items.map(mapPropertyResponse),
    suspendedShared: response.suspended_shared
      ? mapSuspendedShared(response.suspended_shared)
      : [],
    today: response.today,
  };
}

/** Конфиг справочника объектов (канон #887) — возвращаемый тип фабрики
 * propertiesListQueryOptions: экспорты features/ несут явные возвращаемые
 * типы (apps/frontend/AGENTS.md), члены — их выведенная форма. */
export type PropertiesListQueryConfig = {
  readonly queryKey: typeof propertyKeys.list;
  readonly queryFn: () => Promise<PropertiesListResult>;
};

/** Опции справочника объектов (канон #887): один источник ключ+fetch для
 * хуков, прогрева хабов #626 и серверного префетча. */
export function propertiesListQueryOptions(
  transport: ApiTransport = apiClient,
): PropertiesListQueryConfig {
  return {
    queryKey: propertyKeys.list,
    queryFn: () => fetchProperties(transport),
  };
}

/** Чистый fetch детали объекта — общее горло хука и серверного префетча
 * #887. */
export async function fetchProperty(
  id: string,
  transport: ApiTransport = apiClient,
): Promise<Property> {
  const response = await transport<PropertyResponse>(`/properties/${id}`);
  return mapPropertyResponse(response);
}

/** Опции детали объекта (канон #887): один источник ключ+fetch для хука
 * и серверного префетча; гейт-запрос страницы объекта. */
export function propertyDetailQueryOptions({
  id,
  transport = apiClient,
}: {
  readonly id: string;
  readonly transport?: ApiTransport;
}): UseQueryOptions<Property, ApiError, Property, ReturnType<typeof propertyKeys.detail>> {
  return queryOptions({
    queryKey: propertyKeys.detail(id),
    queryFn: () => fetchProperty(id, transport),
  });
}

/** Загрузка/замена фото объекта — POST /properties/{id}/photo multipart-
 * формой с полем `file` (ADR 0065); ответ — обновлённый объект с новым
 * photoUrl, валидация и EXIF-стриж на бэкенде. */
export async function uploadPropertyPhoto(
  { id, file }: { readonly id: string; readonly file: File },
  transport: ApiTransport = apiClient,
): Promise<Property> {
  const body = new FormData();
  body.append('file', file);
  const response = await transport<PropertyResponse>(`/properties/${id}/photo`, {
    method: 'POST',
    body,
  });
  return mapPropertyResponse(response);
}

/** Удаление фото объекта — DELETE /properties/{id}/photo (204 без тела,
 * ADR 0065). */
export async function deletePropertyPhoto(
  { id }: { readonly id: string },
  transport: ApiTransport = apiClient,
): Promise<void> {
  await transport<void>(`/properties/${id}/photo`, { method: 'DELETE' });
}
