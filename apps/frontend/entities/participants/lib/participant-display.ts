import type { Participant } from '../model/types';
import type { ParticipantRowBadge } from './participant-badge';

/** Чип статуса — общий ParticipantRowBadge (анатомия «Row Button Bage»,
 * Figma 2036:83107). */
export type ParticipantBadge = ParticipantRowBadge;

/** Warning-чип превышенного лимита (макет 3313-77128, Row Button Bage
 * Warning): жёлтая подложка, Icon/S/Lock. Именованный конструктор вместо
 * ручного литерала — текст и тон держатся в одном месте. */
export const suspendedLimitBadge = (): ParticipantRowBadge => ({
  tone: 'warning',
  label: 'Превышен лимит объектов',
  icon: 'lock',
});

/** Info-чип pending-приглашения (D6 #1312, макет 3298-75434): голубая
 * подложка, Icon/S/Lock, «Аккаунт не создан». Именованный конструктор —
 * как suspendedLimitBadge: текст и тон в одном месте. */
export const accountNotCreatedBadge = (): ParticipantRowBadge => ({
  tone: 'info',
  label: 'Аккаунт не создан',
  icon: 'lock',
});

/** Титул строки списка: имя; у pending-строки (имени нет) — почта
 * (контракт #693: «the email is the label»). */
export function participantRowTitle(participant: Participant): string {
  return participant.displayName ?? participant.email ?? '';
}

/** Подзаголовок строки: почта; у pending-строки почта уже титул — дважды
 * не повторяется (макет 2036-82971: title + email-подзаголовок). */
export function participantRowSubtitle(participant: Participant): string | undefined {
  if (participant.displayName === undefined) {
    return undefined;
  }
  return participant.email;
}

/** Pending-агрегат — незарегистрированный приглашённый (идентификатор —
 * почта, юзера нет), у которого нет ни одной активной или подвесшей ноги:
 * доступа пока нет, есть только приглашение. На проводе такое состояние
 * совпадает с `user_id = null` (registered-бакет собирается только из
 * memberships), проверка ног — страховка от ложного бейджа. */
function isPendingAggregate(participant: Participant): boolean {
  return (
    participant.userId === undefined
    && participant.properties.every((leg) => leg.status === 'pending')
  );
}

/** Чип статуса строки (макет 3313-77128, D3): ровно два статусных бейджа —
 * pending-агрегат «Аккаунт не создан» и «Превышен лимит объектов»
 * (suspended). Статусы «частично/все объекты» бейджа не имеют — счётчики
 * и градации доступа ушли с экранов раздела (#1313); null — строки без
 * бейджа. */
export function participantStatusBadge(participant: Participant): ParticipantBadge | null {
  if (isPendingAggregate(participant)) {
    return accountNotCreatedBadge();
  }
  if (participant.aggregateStatus === 'limit_exceeded') {
    return suspendedLimitBadge();
  }
  return null;
}
