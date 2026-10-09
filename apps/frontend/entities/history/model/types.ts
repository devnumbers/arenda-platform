/**
 * Доменные модели слайса «История действий» (карта #704, ADR 0061):
 * снимок одной записи журнала и опции шита фильтров. Контракт GET /history
 * и /history/filters (#708) — snake_case; перевод в camelCase — забота
 * mappers.ts.
 */

import type { components } from '@/shared/api/dto';

/** Роль актёра на объекте в момент действия (снимок, ADR 0061 §4). */
export type HistoryActorRole = 'owner' | 'full_access' | 'viewer';

/** Вид действия — одна из семи групп словаря журнала (ADR 0061 §4). */
export type HistoryKind =
  | 'property'
  | 'rental'
  | 'payment'
  | 'operation'
  | 'contact'
  | 'task'
  | 'member';

/** Основное действие — группа фильтра, иконка и цвет строки (ADR 0061 §4). */
export type HistoryBaseAction = 'added' | 'changed' | 'completed' | 'deleted';

/** Ссылка фрагмента строки на страницу сущности (синие переходы, тикет #713). */
export type HistorySegmentLink = {
  readonly kind: HistoryKind;
  readonly id: string;
};

/**
 * Фрагмент текста строки, собранный сервером при записи (ADR 0061 §6):
 * экран рендерит фрагменты дословно, связанный фрагмент оборачивается
 * синим переходом.
 */
export type HistorySegment = {
  readonly text: string;
  readonly link?: HistorySegmentLink;
};

/** Одна запись ленты — снимок действия при записи (ADR 0061 §3–§6). */
export type HistoryEntry = {
  readonly id: string;
  readonly propertyId: string;
  /** Название объекта для шапки группы (живое — читается при чтении). */
  readonly propertyName: string;
  /** null — пользователь удалён, запись обезличена (снимки остаются). */
  readonly actorId: string | null;
  readonly actorName: string;
  /** Путь стриминга фото профиля актёра (ADR 0065, решение #1286), живой
   * на чтение; null — у актёра нет фото или запись обезличена. */
  readonly actorPhotoUrl: string | null;
  readonly actorRole: HistoryActorRole;
  readonly baseAction: HistoryBaseAction;
  /** Стабильный точечный id действия, например operation.paid. */
  readonly action: string;
  readonly segments: ReadonlyArray<HistorySegment>;
  /** Момент действия, ISO-instant; группировка дней — дело экрана. */
  readonly createdAt: string;
};

/** Участник-вариант шита фильтров (GET /history/filters, #708). firstName —
 * имя без фамилии для строки «(Вы)» ('' — имени нет); isOwner — владелец
 * хотя бы одного объекта области чтения (иконка-замок, макет 2067-163528);
 * role — максимальная роль в области (иконка роли строки, макет
 * 2184-94261: owner — замок, full_access — силуэт, viewer — глаз). */
export type HistoryParticipantOption = {
  readonly id: string;
  readonly name: string;
  readonly email: string;
  readonly firstName: string;
  /** Путь стриминга фото профиля (ADR 0065, решение #1286); null — фото
   * нет. Отозванный участник остаётся опцией: его путь отвечает 404. */
  readonly photoUrl: string | null;
  readonly isOwner: boolean;
  readonly role: 'owner' | 'full_access' | 'viewer';
};

/** Объект-вариант шита фильтров с фото-аватаром ('' — фото нет). type —
 * ключ глифа-плейсхолдера аватара (карта #1217, #1244; словарь DTO —
 * структурно тот же PropertyType слоя entities/property). */
export type HistoryObjectOption = {
  readonly id: string;
  readonly name: string;
  readonly address: string;
  readonly photoUrl: string;
  readonly type: components['schemas']['PropertyType'];
};

/** Опции шита фильтров области чтения (GET /history/filters, #708). */
export type HistoryFilterOptions = {
  readonly participants: ReadonlyArray<HistoryParticipantOption>;
  readonly objects: ReadonlyArray<HistoryObjectOption>;
};
