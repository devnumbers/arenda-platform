/**
 * Карточка книги контактов (ADR 0051): человек для объекта — не пользователь
 * сервиса. Необязательные текстовые поля приходят с сервера пустыми строками
 * (контракт #507), поэтому здесь они неопциональные.
 */
export type Contact = {
  readonly id: string;
  /** Привязка к объекту; undefined — «без объекта» (wire null). */
  readonly propertyId: string | undefined;
  readonly firstName: string;
  readonly lastName: string;
  readonly patronymic: string;
  /** Кем человек приходится объекту — свободный текст, не роль доступа. */
  readonly role: string;
  readonly phone: string;
  readonly email: string;
  readonly messengerName: string;
  readonly messengerUsername: string;
  readonly note: string;
  readonly createdAt: string;
  readonly updatedAt: string;
};

/**
 * Черновик создания карточки (контракт #507, POST /contacts): camelCase
 * 1:1 с ContactCreateRequest. Тексты — уже нормализованные (трим, пустые
 * строки вместо отсутствующих), телефон — в каноническом +7XXXXXXXXXX,
 * привязка — uuid объекта или null («без объекта»).
 */
export type ContactCreateCommand = {
  readonly propertyId: string | null;
  readonly firstName: string;
  readonly lastName: string;
  readonly patronymic: string;
  readonly role: string;
  readonly phone: string;
  readonly email: string;
  readonly messengerName: string;
  readonly messengerUsername: string;
  readonly note: string;
};

/**
 * Черновик правки карточки (контракт #507, PATCH /contacts/{id}): тот же
 * состав, что у создания. Поля необязательные на проводе (опущенное
 * сохраняет значение), но форма #510 всегда отдаёт полный состав —
 * пустая строка явным образом очищает текст, propertyId: null снимает
 * привязку («без объекта»).
 */
export type ContactUpdateCommand = ContactCreateCommand;
