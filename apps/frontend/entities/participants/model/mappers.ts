import type { components } from '@/shared/api/dto';
import type {
  Participant,
  ParticipantPropertyLeg,
} from './types';

type ParticipantDto = components['schemas']['ParticipantResponse'];
type ParticipantPropertyDto = components['schemas']['ParticipantPropertyResponse'];

/** Нога доступа DTO → entity (snake → camel, wire null → undefined). */
function mapLeg(dto: ParticipantPropertyDto): ParticipantPropertyLeg {
  return {
    propertyId: dto.property_id,
    title: dto.title,
    role: dto.role,
    status: dto.status,
    type: dto.type,
    photoUrl: dto.photo_url ?? null,
  };
}

/** Агрегат участника GET /participants (контракт #693) → entity. */
export function mapParticipant(dto: ParticipantDto): Participant {
  return {
    id: dto.id,
    userId: dto.user_id ?? undefined,
    email: dto.email ?? undefined,
    displayName: dto.display_name ?? undefined,
    photoUrl: dto.photo_url ?? null,
    aggregateStatus: dto.aggregate_status,
    accessiblePropertiesCount: dto.accessible_properties_count,
    properties: dto.properties.map(mapLeg),
  };
}
