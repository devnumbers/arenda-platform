import type { Lease } from '@/shared/model/lease';

export type TenantContact = {
  readonly id: string;
  readonly ownerId: string;
  readonly name: string;
  readonly surname: string | null;
  readonly patronymic: string | null;
  readonly phone: string | null;
  readonly email: string | null;
  readonly comment: string | null;
  readonly isActive: boolean;
  readonly activeLease?: Lease;
  readonly lastLease?: Lease;
  readonly createdAt: string;
  readonly updatedAt: string;
};

/**
 * Команда создания контакта арендатора (camelCase; wire-формат сериализуется
 * в features/tenant-contacts). Опциональные поля передаются пустой строкой,
 * чтобы очистить их на бэкенде; отсутствующие поля не меняются.
 */
export type TenantContactCreateRequest = {
  readonly name: string;
  readonly surname?: string;
  readonly patronymic?: string;
  readonly phone?: string;
  readonly email?: string;
  readonly comment?: string;
  /** Контекст объекта: контакт создаётся в аккаунте владельца объекта (shared access). */
  readonly propertyId?: string;
};

/** Команда обновления контакта арендатора (camelCase; wire-формат сериализуется в features/tenant-contacts). */
export type TenantContactUpdateRequest = {
  readonly name?: string;
  readonly surname?: string;
  readonly patronymic?: string;
  readonly phone?: string;
  readonly email?: string;
  readonly comment?: string;
};
