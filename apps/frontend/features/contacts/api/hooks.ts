'use client';

import { useQuery, type UseQueryResult } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import type { ApiError } from '@/shared/api/errors';
import { mapContact } from '@/entities/contact';
import type { Contact } from '@/entities/contact';
import { contactKeys } from '@/shared/api/query-keys';
import type { components } from '@/shared/api/dto';

type ContactsResponse = components['schemas']['ContactsResponse'];

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
