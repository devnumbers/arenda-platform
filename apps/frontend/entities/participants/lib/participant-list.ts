import type { Participant } from '../model/types';
import { participantRowTitle } from './participant-display';

/** Направление чип-сортировки «Имя» (макет 2036-82971). */
export type ParticipantSortOrder = 'asc' | 'desc';

/** Локальная сортировка по имени: русская коллация, регистр не важен
 * (прецедент contactSortByName). Дефолт сервера — name ASC (#693), asc
 * идемпотентен, desc переворачивает. */
const nameCollator = new Intl.Collator('ru');

export function sortParticipantsByName(
  participants: ReadonlyArray<Participant>,
  order: ParticipantSortOrder,
): Participant[] {
  return [...participants].sort(
    (a, b) =>
      nameCollator.compare(participantRowTitle(a), participantRowTitle(b)) *
      (order === 'asc' ? 1 : -1),
  );
}

/**
 * Клиентский поиск по списку (иконка в шапке; объём мал — список
 * GET /participants приходит целиком, без пагинации, канон серверного
 * поиска #601 про большие ленты): подстрока без регистра по титулу
 * строки (имя или почта pending) и по почте.
 */
export function filterParticipantsByQuery(
  participants: ReadonlyArray<Participant>,
  query: string,
): Participant[] {
  const needle = query.trim().toLowerCase();
  if (needle.length === 0) {
    return [...participants];
  }
  return participants.filter((participant) => {
    const title = participantRowTitle(participant).toLowerCase();
    const email = participant.email?.toLowerCase() ?? '';
    return title.includes(needle) || email.includes(needle);
  });
}
