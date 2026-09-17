import type { components } from '@/shared/api/dto';
import type { ParticipantAccessRole } from '@/entities/participants';

type ParticipantPropertiesAddRequest =
  components['schemas']['ParticipantPropertiesAddRequest'];

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
