import type { components } from '@/shared/api/dto';
import type { ParticipantAccessRole } from '@/entities/participants';

type ParticipantPropertiesAddRequest =
  components['schemas']['ParticipantPropertiesAddRequest'];
type ParticipantInviteRequest =
  components['schemas']['ParticipantInviteRequest'];

/** Команда «Пригласить в объект» (#698, POST
 * /participants/{participantId}/properties #694): одна роль на выбранные
 * объекты — снапшот выбора на момент подтверждения. */
export type AddParticipantPropertiesCommand = {
  readonly role: ParticipantAccessRole;
  readonly propertyIds: readonly string[];
};

/** Entity (camelCase) → wire (snake_case): явный сериализатор контракта
 * #694; дубликаты id сворачивает бэк (первое вхождение). */
export function toAddParticipantPropertiesWireRequest(
  command: AddParticipantPropertiesCommand,
): ParticipantPropertiesAddRequest {
  return {
    role: command.role,
    property_ids: [...command.propertyIds],
  };
}

/** Команда приглашения участника из хаба (#699, POST /participants/invite
 * #694): одна почта и одна роль на снапшот выбранных объектов.
 * Зарегистрированная почта получает членства сразу (per-property слоты),
 * незарегистрированная — pending-приглашения и одно письмо со списком. */
export type InviteParticipantCommand = {
  readonly email: string;
  readonly role: ParticipantAccessRole;
  readonly propertyIds: readonly string[];
};

/** Entity → wire: сериализатор контракта #694; дубликаты id сворачивает
 * бэк (первое вхождение). */
export function toInviteParticipantWireRequest(
  command: InviteParticipantCommand,
): ParticipantInviteRequest {
  return {
    email: command.email,
    role: command.role,
    property_ids: [...command.propertyIds],
  };
}

/** Результат исхода партии грантов: granted = активные + suspended
 * (слоты получателя) + pending (письмо) — человек приглашён в любом из
 * этих исходов; остальные — skipped_*. */
export type ParticipantGrantOutcomeCounts = {
  readonly granted: number;
  readonly skipped: number;
};

/** Классификация исходов партии (зеркало ParticipantGrantOutcome бэка,
 * #694): единственное место, знающее, какие исходы считаются грантом;
 * оба mutationFn «Пригласить в объект» и приглашения хаба считают им. */
export function countGrantOutcomes(
  items: ReadonlyArray<{ readonly outcome: string }>,
): ParticipantGrantOutcomeCounts {
  let granted = 0;
  let skipped = 0;
  for (const item of items) {
    if (item.outcome === 'active' || item.outcome === 'suspended' || item.outcome === 'pending') {
      granted += 1;
    } else {
      skipped += 1;
    }
  }
  return { granted, skipped };
}
