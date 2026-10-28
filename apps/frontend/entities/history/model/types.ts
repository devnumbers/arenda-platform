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
  /** Живое имя актёра при чтении (карта #1324, D5, паттерн ADR 0065);
   * '' — имени нет или запись обезличена. Сборку отображаемого имени
   * делает фронт (хелпер D4) — сервер отдаёт части. */
  readonly actorFirstName: string;
  /** Живая фамилия актёра при чтении (D5); '' — фамилии нет или запись
   * обезличена. */
  readonly actorLastName: string;
  /** Путь стриминга фото профиля актёра (ADR 0065, решение #1286), живой
   * на чтение; null — у актёра нет фото или запись обезличена. */
  readonly actorPhotoUrl: string | null;
  /** Blur-кадр аватара актёра (ADR 0065 аменд, решение #1594); null — кадра нет. */
  readonly actorPhotoBlur: string | null;
  readonly actorRole: HistoryActorRole;
  readonly baseAction: HistoryBaseAction;
  /** Стабильный точечный id действия, например operation.paid. */
  readonly action: string;
  readonly segments: ReadonlyArray<HistorySegment>;
  /** Серые строки-пояснения нового шаблона (карта #1324, D1): собраны
   * сервером при записи, рендерятся дословно под заголовком. undefined —
   * строка без пояснений (записанные до D1 и действия без диффов) —
   * рендерится одним заголовком (решение владельца). */
  readonly details?: ReadonlyArray<string>;
  /** Момент действия, ISO-instant; группировка дней — дело экрана. */
  readonly createdAt: string;
};

/** Участник-вариант шита фильтров (GET /history/filters, #708). firstName —
 * имя без фамилии для строки «(Вы)» ('' — имени нет); isOwner — владелец
 * хотя бы одного объекта области чтения (замок строки шита — единственная
 * иконка участника, решение владельца 12.10.2026 #1360, макет
 * 3339-160075); role — максимальная роль в области; status —
 * живая нога в области (карта #1324, D6): current — владелец или членство
 * (включая suspended), removed — только строки журнала (ревок/выход). */
export type HistoryParticipantOption = {
  readonly id: string;
  readonly name: string;
  readonly email: string;
  readonly firstName: string;
  /** Путь стриминга фото профиля (ADR 0065, решение #1286); null — фото
   * нет. Отозванный участник остаётся опцией: его путь отвечает 404. */
  readonly photoUrl: string | null;
  /** Blur-кадр фото первого показа (ADR 0065 аменд, решение #1594); null — кадра нет. */
  readonly photoBlur: string | null;
  readonly isOwner: boolean;
  readonly role: 'owner' | 'full_access' | 'viewer';
  readonly status: 'current' | 'removed';
};

/** Объект-вариант шита фильтров с фото-аватаром ('' — фото нет). type —
 * ключ глифа-плейсхолдера аватара (карта #1217, #1244; словарь DTO —
 * структурно тот же PropertyType слоя entities/property). */
export type HistoryObjectOption = {
  readonly id: string;
  readonly name: string;
  readonly address: string;
  readonly photoUrl: string;
  /** Blur-кадр фото первого показа (ADR 0065 аменд, решение #1594);
   * null — кадра нет. */
  readonly photoBlur: string | null;
  readonly type: components['schemas']['PropertyType'];
};

/** Опции шита фильтров области чтения (GET /history/filters, #708). */
export type HistoryFilterOptions = {
  readonly participants: ReadonlyArray<HistoryParticipantOption>;
  readonly objects: ReadonlyArray<HistoryObjectOption>;
};
