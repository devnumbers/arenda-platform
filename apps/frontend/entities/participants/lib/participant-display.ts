import { pluralize } from '@/shared/lib/pluralize';
import type { Participant } from '../model/types';
import type { ParticipantRowBadge } from './participant-badge';

/** Чип агрегат-статуса — общий ParticipantRowBadge (анатомия «Row Button
 * Bage», Figma 2036:83107). */
export type ParticipantBadge = ParticipantRowBadge;

/** Warning-чип превышенного лимита (макет 2036-82971, Row Button Bage
 * Warning): жёлтая подложка, Icon/S/Lock. Именованный конструктор вместо
 * ручного литерала — текст и тон держатся в одном месте. */
export const suspendedLimitBadge = (): ParticipantRowBadge => ({
  tone: 'warning',
  label: 'Превышен лимит объектов',
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
 * memberships), проверка ног — страховка от ложного «Приглашён». */
function isPendingAggregate(participant: Participant): boolean {
  return (
    participant.userId === undefined
    && participant.properties.every((leg) => leg.status === 'pending')
  );
}

/** Чип агрегат-статуса: pending-агрегат — «Приглашён» (решение владельца
 * Q11=А, #772: считанный partial/0 как «Доступно 0 объектов» читался
 * как отказ); зарегистрированным — подписи контракта #693: «Доступ ко
 * всем объектам» / «Доступно N объектов» / «Превышен лимит объектов». */
export function participantStatusBadge(participant: Participant): ParticipantBadge {
  if (isPendingAggregate(participant)) {
    return { tone: 'neutral', label: 'Приглашён' };
  }
  switch (participant.aggregateStatus) {
    case 'limit_exceeded':
      return suspendedLimitBadge();
    case 'all_properties':
      return { tone: 'neutral', label: 'Доступ ко всем объектам' };
    case 'partial':
      return {
        tone: 'neutral',
        label: `Доступно ${participant.accessiblePropertiesCount} ${pluralize(
          participant.accessiblePropertiesCount,
          'объект',
          'объекта',
          'объектов',
        )}`,
      };
  }
}
