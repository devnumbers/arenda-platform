/**
 * DTO → entity слайса контактов. Контракт #507 camelCase с пустыми строками
 * вместо null в текстовых полях, поэтому маппер только нормализует nullable
 * привязку к объекту к опциональности.
 */

import type { components } from '@/shared/api/dto';
import type { Contact } from './types';

type ContactDto = components['schemas']['ContactResponse'];

export function mapContact(dto: ContactDto): Contact {
  return {
    id: dto.id,
    propertyId: dto.propertyId ?? undefined,
    propertyName: dto.propertyName ?? undefined,
    firstName: dto.firstName,
    lastName: dto.lastName,
    patronymic: dto.patronymic,
    role: dto.role,
    phone: dto.phone,
    email: dto.email,
    messengerName: dto.messengerName,
    messengerUsername: dto.messengerUsername,
    note: dto.note,
    createdAt: dto.createdAt,
    updatedAt: dto.updatedAt,
  };
}
