import { filterByRuQuery, sortByRuText } from './participant-sorting';
import type { ParticipantSortOrder } from './participant-sorting';
import type { Participant } from '../model/types';
import { participantRowTitle } from './participant-display';

export type { ParticipantSortOrder } from './participant-sorting';

/** Сортировка по имени: русская коллация без регистра. Дефолт сервера —
 * name ASC (#693), asc идемпотентен, desc переворачивает. */
export function sortParticipantsByName(
  participants: ReadonlyArray<Participant>,
  order: ParticipantSortOrder,
): Participant[] {
  return sortByRuText(participants, participantRowTitle, order);
}

/** Клиентский поиск по списку (иконка в шапке): подстрока без регистра
 * по титулу строки (имя или почта pending) и по почте. */
export function filterParticipantsByQuery(
  participants: ReadonlyArray<Participant>,
  query: string,
): Participant[] {
  return filterByRuQuery(
    participants,
    (participant) => `${participantRowTitle(participant)} ${participant.email ?? ''}`,
    query,
  );
}
