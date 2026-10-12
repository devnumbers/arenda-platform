import type { AccessRole, PropertyAccessMember } from '@/entities/access';
import { ACCESS_ROLE_LABELS } from '@/entities/access';
import { filterByRuQuery, sortByRuText } from '@/entities/participants';
import { displayUserName } from '@/shared/lib/display-name';
import { parseEnumParam } from '@/shared/lib/parse-enum-param';

/** Иконка у почты в ряду (макет 1980-107096): владелец — замок,
 * full_access — перо, viewer — глаз. */
export type PropertyParticipantEmailIcon = 'owner' | 'edit' | 'eye';

/** Значение чип-фильтра «Все роли» (макет 1980-109148). */
export type PropertyParticipantRoleFilter = 'all' | Exclude<AccessRole, 'owner'>;

/** Направление чипа «Имя» — как сортировка «Ваших участников» (#697). */
import type { ParticipantSortOrder as PropertyParticipantSortOrder } from '@/entities/participants';

export type { PropertyParticipantSortOrder };

/** Подписи чипа и опций фильтра ролей: «Все роли» плюс канон ролей
 * ACCESS_ROLE_LABELS (единственный источник подписи роли, shared/model/
 * access) — второй копии литералов здесь нет. */
export const ROLE_FILTER_LABELS: Record<PropertyParticipantRoleFilter, string> = {
  all: 'Все роли',
  viewer: ACCESS_ROLE_LABELS.viewer,
  full_access: ACCESS_ROLE_LABELS.full_access,
};

/** Дефолты чипов «Имя» и «Все роли» — в адресе не живут (конвенция
 * состояния в адресе, #785). */
export const DEFAULT_PROPERTY_PARTICIPANT_ORDER: PropertyParticipantSortOrder = 'asc';
export const DEFAULT_PROPERTY_PARTICIPANT_ROLE_FILTER: PropertyParticipantRoleFilter = 'all';

/** Разбор ?order= экрана «Участники объекта»: неизвестное и отсутствующее
 * значения — дефолт «А→Я». */
export function parsePropertyParticipantOrderParams(
  order: string | string[] | undefined,
): PropertyParticipantSortOrder {
  return parseEnumParam(order, ['asc', 'desc'], DEFAULT_PROPERTY_PARTICIPANT_ORDER);
}

/** Разбор ?role= фильтра ролей: неизвестное (в том числе owner — роли ног
 * только viewer/full_access) и отсутствующее — «Все роли». */
export function parsePropertyParticipantRoleFilterParams(
  role: string | string[] | undefined,
): PropertyParticipantRoleFilter {
  return parseEnumParam(
    role,
    ['all', 'viewer', 'full_access'],
    DEFAULT_PROPERTY_PARTICIPANT_ROLE_FILTER,
  );
}

/** Собственные параметры чипов в адресе — знание этого модуля; писатель
 * (PropertyParticipantsScreen) импортирует отсюда: у каждого чипа свой
 * ключ, чужой при записи не трогается. */
export const PROPERTY_PARTICIPANT_ORDER_PARAMS = ['order'] as const;
export const PROPERTY_PARTICIPANT_ROLE_FILTER_PARAMS = ['role'] as const;

/** Патчи чипов для адреса: дефолтные значения параметров не создают
 * (пустой query — голый pathname); пишутся через useUrlParams с own
 * (#785). */
export function serializePropertyParticipantOrderToParams(
  order: PropertyParticipantSortOrder,
): Record<string, string> {
  return order === DEFAULT_PROPERTY_PARTICIPANT_ORDER ? {} : { order };
}

export function serializePropertyParticipantRoleFilterToParams(
  role: PropertyParticipantRoleFilter,
): Record<string, string> {
  return role === DEFAULT_PROPERTY_PARTICIPANT_ROLE_FILTER ? {} : { role };
}

/** Данные ряда списка участников объекта: всё, что рисует строка,
 * вычислено один раз (ViewModel) — экрану остаётся только рендер. */
export type PropertyParticipantRow = {
  /** Стабильный ключ: id строки доступа, fallback — userId/email. */
  readonly key: string;
  /** База титула по ФИО-канону D4: чужие — серверный канон
   * access.DisplayNameOf (почта — только у pending, контракт #693),
   * своя строка — имя читателя без фамилии (макет «Даниил (Вы)»).
   * Суффикс «(Вы)» в базе не живёт — его серым рисует вид по isMe
   * (канон шита фильтров #710). */
  readonly title: string;
  /** Путь стриминга фото профиля участника (ADR 0065, решение #1286);
   * undefined — pending или у юзера нет фото. */
  readonly photoUrl: string | undefined;
  /** Почта; у pending-строки почта уже титул — дважды не повторяется. */
  readonly subtitle: string | undefined;
  readonly emailIcon: PropertyParticipantEmailIcon;
  /** Приостановленный доступ — warning-чип «Превышен лимит объектов». */
  readonly suspended: boolean;
  /** Цель тапа — агрегат /participants/{id}: uuid юзера, у pending —
   * почта (контракт #693); undefined — строка без идентификатора. */
  readonly participantId: string | undefined;
  /** Роль ноги: фильтр «Все роли» и сортировка. */
  readonly role: AccessRole;
  readonly isOwner: boolean;
  /** Строка самого читателя — вид рисует серый суффикс «(Вы)» (канон
   * #710); ряд инертный (#770; политика тапа — в доке экрана). */
  readonly isMe: boolean;
};

/** Читатель списка — источник своей строки (isMe): uuid сравнивается с
 * ногами, имя собирает канон D4 («Имя», без имени — «Пользователь»).
 * Структурно совместим с User из /me — экран передаёт его напрямую. */
export type PropertyParticipantsReader = {
  readonly id: string;
  /** Имя профиля (first name); null — имени нет. */
  readonly name: string | null;
};

/**
 * Ряды списка «Участники объекта» (карта #692, тикет #700) из строк
 * GET /properties/{id}/access/members: владелец — отдельным первым рядом
 * (макет 1980-107096), отметка «(Вы)» — у строки текущего читателя.
 * Порядок входа участников сохраняется — сортировка отдельно. Почта есть
 * у каждой строки контракта, чей адрес разрешился (решение владельца
 * 2026-09-20, обход #758: зрителю почты участников тоже видны).
 */
export function propertyParticipantRows(
  members: ReadonlyArray<PropertyAccessMember>,
  reader: PropertyParticipantsReader | undefined,
): PropertyParticipantRow[] {
  const rows = members.map(toPropertyParticipantRow(reader));
  // Инвариант макета: ряд владельца первым независимо от порядка ответа.
  const owner = rows.find((row) => row.isOwner);
  return owner === undefined ? rows : [owner, ...rows.filter((row) => !row.isOwner)];
}

function toPropertyParticipantRow(reader: PropertyParticipantsReader | undefined) {
  return (member: PropertyAccessMember): PropertyParticipantRow => {
    const email = member.email ?? '';
    const displayName = member.displayName !== '' ? member.displayName : undefined;
    const isMe = reader !== undefined && member.userId === reader.id;
    // ФИО-канон D4: своя строка — «Имя (Вы)» — имя читателя из /me без
    // фамилии серверной строки (прецедент шита фильтров #710); нет
    // имени — «Пользователь» (даст на виде «Пользователь (Вы)»). Чужие —
    // серверный канон DisplayNameOf, фронт его не пересобирает; почта в
    // титуле по контракту бывает только у pending («the email is the
    // label», #693) — фолбэк ниже для зарегистрированных недостижим
    // (канон гарантирует display_name), это защита от пустого титула.
    const title = isMe
      ? displayUserName(reader.name ?? '', '')
      : (displayName ?? (email !== '' ? email : ''));
    return {
      key: member.id ?? member.userId ?? (email !== '' ? email : 'unknown'),
      title,
      photoUrl: member.photoUrl ?? undefined,
      subtitle: displayName !== undefined && email !== '' ? email : undefined,
      emailIcon: member.isOwner ? 'owner' : member.role === 'full_access' ? 'edit' : 'eye',
      suspended: member.status === 'suspended',
      participantId:
        member.userId ?? (member.status === 'pending' && email !== '' ? email : undefined),
      role: member.role,
      isOwner: member.isOwner,
      isMe,
    };
  };
}

/** Клиентский поиск по имени и почте (подсказка макета 1980-108531:
 * «Введите имя или почту участника») — объём мал, серверного ?search= нет
 * (прецедент «Ваших участников» #697). */
export function filterPropertyParticipantsByQuery(
  rows: ReadonlyArray<PropertyParticipantRow>,
  query: string,
): PropertyParticipantRow[] {
  return filterByRuQuery(rows, (row) => `${row.title} ${row.subtitle ?? ''}`, query);
}

/** Чип-фильтр «Все роли» (макет 1980-109148): владельцу фильтр не
 * соответствует — остаётся только при «Все роли». */
export function filterPropertyParticipantsByRole(
  rows: ReadonlyArray<PropertyParticipantRow>,
  role: PropertyParticipantRoleFilter,
): PropertyParticipantRow[] {
  if (role === 'all') {
    return [...rows];
  }
  return rows.filter((row) => row.role === role);
}

/** Чип «Имя» (asc/desc): участники сортируются по титулу (русская
 * коллация), ряд владельца не участвует и всегда первый. */
export function sortPropertyParticipantsByTitle(
  rows: ReadonlyArray<PropertyParticipantRow>,
  order: PropertyParticipantSortOrder,
): PropertyParticipantRow[] {
  const owner = rows.find((row) => row.isOwner);
  const participants = sortByRuText(
    rows.filter((row) => !row.isOwner),
    (row) => row.title,
    order,
  );
  return owner !== undefined ? [owner, ...participants] : participants;
}
