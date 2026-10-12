import type { PropertyAccessMember } from '@/entities/access';
import {
  accountNotCreatedBadge,
  type Participant,
} from '@/entities/participants';
import { ANONYMOUS_TITLE, type PropertyParticipantEmailIcon } from './property-participants-list';

/** Данные карточки участника на D7-экране «Права участника» без аккаунта
 * (макет 3298-75576): анатомия Row Button со страницы — аватар, титул,
 * почта с иконкой роли, чип статуса. */
export type ParticipantRightsCard = {
  /** Анонимный титул pending-приглашения (канон #1105, как в рядах D6). */
  readonly title: string;
  readonly email: string;
  /** Иконка роли у почты (дискриминация рядов D6, 3298-75434). */
  readonly emailIcon: PropertyParticipantEmailIcon;
  readonly badge: ReturnType<typeof accountNotCreatedBadge>;
  /** Путь фото профиля для аватара; undefined — у pending его не бывает,
   * поле держит аватар карточки в одном месте с остальными её полями. */
  readonly photoUrl: string | undefined;
};

/**
 * D7-кейс экрана «Права участника» — приглашённый без аккаунта: участник
 * без uuid юзера и строка доступа — pending-приглашение (нечего менять
 * через members — только invitations). Возвращает карточку для рендера
 * или undefined — не D7 (зарегистрированный остаётся на макете
 * 2177-59620; строки нет — доступ уже снят, честный 404 выше).
 */
export function pendingParticipantRightsCard(
  participant: Participant,
  member: PropertyAccessMember | undefined,
): ParticipantRightsCard | undefined {
  if (
    member === undefined ||
    participant.userId !== undefined ||
    member.status !== 'pending'
  ) {
    return undefined;
  }
  return {
    title: ANONYMOUS_TITLE,
    email: participant.email ?? member.email ?? '',
    emailIcon: member.role === 'viewer' ? 'eye' : 'full',
    badge: accountNotCreatedBadge(),
    photoUrl: participant.photoUrl ?? undefined,
  };
}
