/** Агрегат-статус участника (issue #693): «доступ ко всем объектам» /
 * «доступно N объектов» / «превышен лимит объектов» (хоть один suspended). */
export type ParticipantAggregateStatus =
  | 'all_properties'
  | 'partial'
  | 'limit_exceeded';

/** Роль ноги доступа на конкретном объекте (контракт #693). */
export type ParticipantAccessRole = 'full_access' | 'viewer';

/** Жизненный цикл ноги доступа: активное членство, приостановленное
 * (тарифный лимит получателя) или pending-приглашение по email. */
export type ParticipantAccessStatus = 'active' | 'suspended' | 'pending';

/** Одна нога доступа участника — объект сцопа читающего с per-object
 * ролью и статусом (контракт #693). */
export type ParticipantPropertyLeg = {
  readonly propertyId: string;
  readonly title: string;
  readonly role: ParticipantAccessRole;
  readonly status: ParticipantAccessStatus;
};

/**
 * Участник (владельца) — человек во всех доступах к объектам одного
 * владельца: агрегат над property_members ∪ invitations без собственной
 * таблицы (решение чарта карты #692; термины — internal/access/CONTEXT.md).
 * Идентификатор — uuid юзера либо pending-почта.
 */
export type Participant = {
  readonly id: string;
  /** uuid зарегистрированного юзера; undefined — pending-строка. */
  readonly userId: string | undefined;
  /** Почта pending-приглашения; у зарегистрированного — почта для показа
   * (когда у юзера она есть). */
  readonly email: string | undefined;
  /** Имя для показа (имя и фамилия или маскированный телефон); undefined —
   * pending-строка (лейбл строки — почта). */
  readonly displayName: string | undefined;
  readonly aggregateStatus: ParticipantAggregateStatus;
  /** Число объектов с активным доступом. */
  readonly accessiblePropertiesCount: number;
  /** Ноги доступа на объектах сцопа читающего. */
  readonly properties: ReadonlyArray<ParticipantPropertyLeg>;
};
