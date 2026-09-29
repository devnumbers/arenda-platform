import type { UseQueryOptions } from '@tanstack/react-query';
import { apiClient, type ApiTransport } from '@/shared/api/client';
import type { ApiError } from '@/shared/api/errors';
import { mapContact } from '@/entities/contact';
import type { Contact } from '@/entities/contact';
import { contactKeys } from '@/shared/api/query-keys';
import {
  keysetNextPageParam,
  type KeysetPage,
  type KeysetPagedQueryConfig,
} from '@/shared/lib/keyset';
import type { components } from '@/shared/api/dto';

type ContactsResponse = components['schemas']['ContactsResponse'];
type ContactResponse = components['schemas']['ContactResponse'];

/** Порция списка контактов (#600): строки плюс keyset-продолжение —
 * opaque-курсор следующей порции, null = порций больше нет. */
export type ContactsPageData = KeysetPage<Contact>;

/**
 * Конфиг keyset-обхода книги контактов — возвращаемый тип билдеров
 * contactsListQuery/contactBookQuery: экспорты features/ несут явные
 * возвращаемые типы (apps/frontend/AGENTS.md), форма — общий
 * KeysetPagedQueryConfig из shared/lib/keyset.
 */
export type ContactsListQueryConfig = KeysetPagedQueryConfig<
  ReturnType<typeof contactKeys.list>,
  ContactsPageData
>;

/** Двойной модуль API-слоя contacts (без 'use client'): чистые fetch-функции
 * и queryOptions-фабрики канона #887 — общий источник ключ+fetch для
 * клиентских хуков, прогрева хабов #626 и серверного префетча. Хуки — в
 * hooks.ts ('use client'), они реэкспортируют оси сортировки книги. */

/** Поле сортировки плоской книги: по имени или по объекту (макет
 * 1726:65136 — шит «Сортировать»). */
export type ContactBookSort = 'name' | 'property';

/** Направление сортировки плоской книги. */
export type ContactBookOrder = 'asc' | 'desc';

/** Порция списка контактов: контракт книги (#600) — порции по 50. */
export const CONTACTS_PAGE_SIZE = 50;

/** Общее горло порции GET /contacts (#600): с propertyId — срез объекта,
 * без — плоская книга; sort/order уходят только отличные от дефолта
 * (сервер нормализует пустые сам; created — серверная ось «свежие сверху»,
 * #847), cursor — keyset-продолжение прошлого ответа, undefined читает
 * с начала. */
async function fetchContactsPage(params: {
  propertyId?: string | null;
  search?: string;
  sort?: ContactBookSort | 'created';
  order?: ContactBookOrder;
  cursor?: string;
  transport?: ApiTransport;
}): Promise<ContactsPageData> {
  const transport = params.transport ?? apiClient;
  const query = new URLSearchParams();
  if (params.propertyId) {
    query.set('property_id', params.propertyId);
  }
  if (params.search) {
    query.set('search', params.search);
  }
  if (params.sort && params.sort !== 'name') {
    query.set('sort', params.sort);
  }
  if (params.order && params.order !== 'asc') {
    query.set('order', params.order);
  }
  query.set('limit', String(CONTACTS_PAGE_SIZE));
  if (params.cursor) {
    query.set('cursor', params.cursor);
  }
  const response = await transport<ContactsResponse>(
    `/contacts?${query.toString()}`,
  );
  return {
    items: response.items.map(mapContact),
    nextCursor: response.nextCursor ?? null,
  };
}


/** Конфиг keyset-обхода списка контактов (#600) — общее горло useContacts,
 * useContactBook и прогрева хабов #626: один ключ, один fetch, одно правило
 * продолжения — прогрев не может разъехаться с экраном. propertyId — срез
 * объекта (null/undefined — плоская книга), search — серверный фильтр,
 * sort/order — серверная ось и направление ('created'desc — пикер арендатора,
 * #847). */
export function contactsListQuery({
  propertyId = null,
  search,
  sort,
  order,
  transport,
}: {
  readonly propertyId?: string | null;
  readonly search: string;
  readonly sort: ContactBookSort | 'created';
  readonly order: ContactBookOrder;
  /** Серверный префетч #887 подставляет serverApiClient; браузер живёт на
   * дефолте (same-origin прокси /api). */
  readonly transport?: ApiTransport;
}): ContactsListQueryConfig {
  return {
    queryKey: contactKeys.list(propertyId, search, sort, order),
    queryFn: ({ pageParam }: { pageParam?: string }) =>
      fetchContactsPage({ propertyId, search, sort, order, cursor: pageParam, transport }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: keysetNextPageParam,
  };
}


/** Конфиг keyset-обхода плоской книги (#600) — частный случай
 * contactsListQuery для потребителей без среза объекта (useContactBook,
 * прогрев хабов #626). */
export function contactBookQuery(
  search = '',
  sort: ContactBookSort = 'name',
  order: ContactBookOrder = 'asc',
): ContactsListQueryConfig {
  return contactsListQuery({ search, sort, order });
}


/** Чистый fetch карточки контакта — общее горло хука и серверного
 * префетча #887. */
export async function fetchContact(
  contactId: string,
  transport: ApiTransport = apiClient,
): Promise<Contact> {
  const response = await transport<ContactResponse>(
    `/contacts/${encodeURIComponent(contactId)}`,
  );
  return mapContact(response);
}

/** Опции карточки контакта (канон #887): один источник ключ+fetch для
 * хука и серверного префетча. */
export function contactDetailQueryOptions({
  contactId,
  transport = apiClient,
}: {
  readonly contactId: string;
  readonly transport?: ApiTransport;
}): UseQueryOptions<Contact, ApiError, Contact, ReturnType<typeof contactKeys.detail>> {
  return {
    queryKey: contactKeys.detail(contactId),
    queryFn: () => fetchContact(contactId, transport),
  };
}

