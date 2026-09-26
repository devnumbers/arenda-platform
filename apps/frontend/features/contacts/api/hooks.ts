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
import { mapContact } from '@/entities/contact';
import type {
  Contact,
  ContactCreateCommand,
  ContactUpdateCommand,
} from '@/entities/contact';
import { contactKeys } from '@/shared/api/query-keys';
import { keysetNextPageParam } from '@/shared/lib/keyset';
import type { components } from '@/shared/api/dto';

type ContactsResponse = components['schemas']['ContactsResponse'];
type ContactResponse = components['schemas']['ContactResponse'];

/** Порция списка контактов: контракт книги (#600) — порции по 50. */
export const CONTACTS_PAGE_SIZE = 50;

/**
 * Порция списка контактов (#600): строки плюс keyset-продолжение —
 * opaque-курсор следующей порции, null = порций больше нет.
 */
export type ContactsPageData = {
  readonly items: Contact[];
  readonly nextCursor: string | null;
};

/**
 * Книга контактов объекта (ADR 0054, экран #508): порции по 50 keyset-курсором
 * (#600) — pageParam это курсор прошлого ответа, без него чтение с начала.
 * search — серверный регистронезависимый подстрочный фильтр по имени,
 * телефону, почте, имени пользователя мессенджера и роли ('' = без фильтра).
 * Чтение через view-гейт объекта: участник с ролью CanView видит привязанные
 * к объекту контакты (403 — проблема Forbidden).
 * keepPreviousData — прежний срез держится на экране, пока едет запрос
 * с новым ?search= (набор не мигает скелетоном, канон платежей #609);
 * смена propertyId держит список прежнего объекта до прихода нового —
 * осознанно, как у платежей.
 */
export function useContacts(
  propertyId: string,
  search = '',
  options: { readonly enabled?: boolean; readonly sort?: 'created' } = {},
): UseInfiniteQueryResult<Contact[], ApiError> {
  // sort едет в ключ кэша: срез 'created' и дефолтный 'name' — разные
  // наборы, один ключ смешал бы их (канон contactKeys.list).
  const sort = options.sort ?? 'name';
  return useInfiniteQuery({
    queryKey: contactKeys.list(propertyId, search, sort),
    queryFn: ({ pageParam }) =>
      fetchContactsPage({ propertyId, search, sort, cursor: pageParam }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: keysetNextPageParam,
    select: (data) => data.pages.flatMap((page) => page.items),
    placeholderData: keepPreviousData,
    enabled: (options.enabled ?? true) && Boolean(propertyId),
  });
}

/** Поле сортировки плоской книги: по имени или по объекту (макет
 * 1726:65136 — шит «Сортировать»). */
export type ContactBookSort = 'name' | 'property';

/** Направление сортировки плоской книги. */
export type ContactBookOrder = 'asc' | 'desc';

/** Общее горло порции GET /contacts (#600): с propertyId — срез объекта,
 * без — плоская книга; sort/order уходят только отличные от дефолта
 * (сервер нормализует пустые сам; created — серверная ось «свежие сверху»,
 * #847), cursor — keyset-продолжение прошлого ответа, undefined читает
 * с начала. */
async function fetchContactsPage(params: {
  propertyId?: string;
  search?: string;
  sort?: ContactBookSort | 'created';
  order?: ContactBookOrder;
  cursor?: string;
}): Promise<ContactsPageData> {
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
  const response = await apiClient<ContactsResponse>(
    `/contacts?${query.toString()}`,
  );
  return {
    items: response.items.map(mapContact),
    nextCursor: response.nextCursor ?? null,
  };
}

/** Конфиг keyset-обхода плоской книги (#600) — общее горло useContactBook
 * и прогрева хабов #626: один ключ, один fetch, одно правило продолжения —
 * прогрев не может разъехаться с экраном. */
export function contactBookQuery(
  search = '',
  sort: ContactBookSort = 'name',
  order: ContactBookOrder = 'asc',
) {
  return {
    queryKey: contactKeys.list(null, search, sort, order),
    queryFn: ({ pageParam }: { pageParam?: string }) =>
      fetchContactsPage({ search, sort, order, cursor: pageParam }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: keysetNextPageParam,
  };
}

/**
 * Плоский список всей видимой книги (глобальная страница контактов, макет
 * 1726:65083): GET /contacts без property_id — сервер отдаёт объединённый
 * срез (свои карточки + привязанные к доступным объектам, ADR 0028/0054) с
 * серверными search/sort/order; propertyName в ответе — имя привязанного
 * объекта для подзаголовков и групп. Порции по 50 листаются keyset-курсором
 * (#600): pageParam — курсор прошлого ответа, смена queryKey начинает свежий
 * обход с пустого курсора — sentinel не наследует позицию прошлых порций.
 * Склейка порций без дедупа: один сортировочный ключ, повторы keyset не
 * порождает (решение тикета #600).
 */
export function useContactBook(
  search = '',
  sort: ContactBookSort = 'name',
  order: ContactBookOrder = 'asc',
): UseInfiniteQueryResult<Contact[], ApiError> {
  return useInfiniteQuery({
    ...contactBookQuery(search, sort, order),
    select: (data: InfiniteData<ContactsPageData>) =>
      data.pages.flatMap((page) => page.items),
    // Набор в поиске и смена сортировки держат прежнюю выдачу, пока едет
    // новый запрос (#609, канон платежей): скелетон — только когда данных
    // нет вовсе.
    placeholderData: keepPreviousData,
  });
}

/**
 * Создание карточки (экран #509): POST /contacts с уже нормализованной
 * командой (трим и телефон собирает contact-form). Инвалидация — вся книга
 * contacts: список объекта читается с серверным ?search=, точечно
 * инвалидовать его нельзя.
 */
export function useCreateContact(): UseMutationResult<
  Contact,
  ApiError,
  ContactCreateCommand
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (command: ContactCreateCommand) => {
      const response = await apiClient<ContactResponse>('/contacts', {
        method: 'POST',
        body: JSON.stringify(command),
      });
      return mapContact(response);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: contactKeys.all });
    },
  });
}

/**
 * Карточка контакта (экран #510): GET /contacts/{id}. 403/404 — состояния
 * экрана (доступ по view-гейту привязанного объекта, ADR 0028).
 */
export function useContact(contactId: string): UseQueryResult<Contact, ApiError> {
  return useQuery({
    queryKey: contactKeys.detail(contactId),
    queryFn: async () => {
      const response = await apiClient<ContactResponse>(
        `/contacts/${encodeURIComponent(contactId)}`,
      );
      return mapContact(response);
    },
    enabled: Boolean(contactId),
  });
}

/**
 * Правка карточки (экран #510): PATCH /contacts/{id} полной командой
 * формы (пустая строка очищает текст, propertyId: null снимает привязку).
 * Инвалидация — вся книга contacts: список объекта читается с серверным
 * ?search=, точечно инвалидовать его нельзя.
 */
export function useUpdateContact(
  contactId: string,
): UseMutationResult<Contact, ApiError, ContactUpdateCommand> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (command: ContactUpdateCommand) => {
      const response = await apiClient<ContactResponse>(
        `/contacts/${encodeURIComponent(contactId)}`,
        { method: 'PATCH', body: JSON.stringify(command) },
      );
      return mapContact(response);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: contactKeys.all });
    },
  });
}

/**
 * Удаление карточки (экран #510): DELETE /contacts/{id}; контакт
 * отвязывается от аренд и объектов на сервере. Кэш книги снимается
 * целиком (deletes — removeQueries, конвенция react-query): данные
 * удалены, списки перечитаются при следующем маунте.
 */
export function useDeleteContact(
  contactId: string,
): UseMutationResult<void, ApiError, void> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      await apiClient<void>(`/contacts/${encodeURIComponent(contactId)}`, {
        method: 'DELETE',
      });
    },
    onSuccess: () => {
      queryClient.removeQueries({ queryKey: contactKeys.all });
    },
  });
}
