import type { PropertyAccessMember } from '@/entities/access';
import type { Participant } from '@/entities/participants';

/**
 * Строка доступа участника на конкретном объекте (экран «Права участника»
 * #698): контракты /participants ногу не адресуют, поэтому participant
 * (uuid либо pending-почта) связывается со строками
 * GET /properties/{id}/access/members — по user_id у зарегистрированного,
 * по email среди pending-строк у приглашения. Владелец объекта (is_owner)
 * участником не является — мимо. Строки нет — доступ уже снят.
 */
export function resolveParticipantMemberRow(
  members: ReadonlyArray<PropertyAccessMember>,
  participant: Participant,
): PropertyAccessMember | undefined {
  return members.find((member) => {
    if (member.isOwner) {
      return false;
    }
    if (participant.userId !== undefined) {
      return member.userId === participant.userId;
    }
    return member.status === 'pending' && member.email === participant.email;
  });
}
