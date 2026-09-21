import type { AccessRole, PropertyAccessMember } from '@/entities/access';

/** Иконка у почты в ряду (макет 1980-107096): владелец — замок,
 * full_access — перо, viewer — глаз. */
export type PropertyParticipantEmailIcon = 'owner' | 'edit' | 'eye';

/** Значение чип-фильтра «Все роли» (макет 1980-109148). */
export type PropertyParticipantRoleFilter = 'all' | Exclude<AccessRole, 'owner'>;

/** Направление чипа «Имя» — как сортировка «Ваших участников» (#697). */
export type PropertyParticipantSortOrder = 'asc' | 'desc';

/** Данные ряда списка участников объекта: всё, что рисует строка,
 * вычислено один раз (ViewModel) — экрану остаётся только рендер. */
export type PropertyParticipantRow = {
  /** Стабильный ключ: id строки доступа, fallback — userId/email. */
  readonly key: string;
  readonly title: string;
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
  /** Строка самого читателя («(Вы)») — ряд инертный (#770; политика
   * тапа — в доке экрана). */
  readonly isMe: boolean;
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
  currentUserId: string | undefined,
): PropertyParticipantRow[] {
  const rows = members.map(toPropertyParticipantRow(currentUserId));
  // Инвариант макета: ряд владельца первым независимо от порядка ответа.
  const owner = rows.find((row) => row.isOwner);
  return owner === undefined ? rows : [owner, ...rows.filter((row) => !row.isOwner)];
}

function toPropertyParticipantRow(currentUserId: string | undefined) {
  return (member: PropertyAccessMember): PropertyParticipantRow => {
    const email = member.email ?? '';
    const displayName = member.displayName !== '' ? member.displayName : undefined;
    const isMe = currentUserId !== undefined && member.userId === currentUserId;
    const title = [displayName ?? (email !== '' ? email : undefined), isMe ? '(Вы)' : undefined]
      .filter(Boolean)
      .join(' ');
    return {
      key: member.id ?? member.userId ?? (email !== '' ? email : 'unknown'),
      title,
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
  const needle = query.trim().toLowerCase();
  if (needle === '') {
    return [...rows];
  }
  return rows.filter((row) =>
    `${row.title} ${row.subtitle ?? ''}`.toLowerCase().includes(needle),
  );
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
  const participants = rows.filter((row) => !row.isOwner);
  const collator = new Intl.Collator('ru');
  participants.sort((a, b) =>
    order === 'asc'
      ? collator.compare(a.title.toLowerCase(), b.title.toLowerCase())
      : collator.compare(b.title.toLowerCase(), a.title.toLowerCase()),
  );
  return owner !== undefined ? [owner, ...participants] : participants;
}
