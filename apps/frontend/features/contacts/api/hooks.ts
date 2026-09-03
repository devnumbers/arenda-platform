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
import { mapContact } from '@/entities/contact';
import type { Contact, ContactCreateCommand } from '@/entities/contact';
import { contactKeys } from '@/shared/api/query-keys';
import type { components } from '@/shared/api/dto';

type ContactsResponse = components['schemas']['ContactsResponse'];
type ContactResponse = components['schemas']['ContactResponse'];

/**
 * Книга контактов объекта (ADR 0051, экран #508): без пагинации, порядок —
 * серверный. search — серверный регистронезависимый подстрочный фильтр по
 * имени, телефону, почте, имени пользователя мессенджера и роли ('' = без
 * фильтра). Чтение через view-гейт объекта: участник с ролью CanView видит
 * привязанные к объекту контакты (403 — проблема Forbidden).
 */
export function useContacts(
  propertyId: string,
  search = '',
): UseQueryResult<Contact[], ApiError> {
  return useQuery({
    queryKey: contactKeys.list(propertyId, search),
    queryFn: async () => {
      const query = search ? `&search=${encodeURIComponent(search)}` : '';
      const response = await apiClient<ContactsResponse>(
        `/contacts?property_id=${encodeURIComponent(propertyId)}${query}`,
      );
      return response.items.map(mapContact);
    },
    enabled: Boolean(propertyId),
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
