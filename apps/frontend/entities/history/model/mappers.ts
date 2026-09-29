/**
 * DTO → доменные модели слайса «История действий». Контракт GET /history
 * и /history/filters (#708) snake_case; опциональные поля нормализуются
 * (actor_id null — запись обезличена). Сегменты рендерятся дословно,
 * кроме голых подписей дат в хвосте (аудит #876: «(срок …)», период
 * аренды) — они срезаются, словесные даты остаются.
 */

import type { components } from '@/shared/api/dto';

import { stripBareDateTails } from './date-tails';
import type {
  HistoryEntry,
  HistoryFilterOptions,
  HistorySegment,
} from './types';

type HistoryItemDto = components['schemas']['HistoryItem'];
type HistoryFiltersDto = components['schemas']['HistoryFiltersResponse'];

function toSegments(dto: HistoryItemDto['segments']): HistorySegment[] {
  return stripBareDateTails(
    dto.map((segment) => ({
      text: segment.text,
      ...(segment.link ? { link: { kind: segment.link.kind, id: segment.link.id } } : {}),
    })),
  );
}

export function mapHistoryItem(dto: HistoryItemDto): HistoryEntry {
  return {
    id: dto.id,
    propertyId: dto.property_id,
    propertyName: dto.property_name,
    actorId: dto.actor_id ?? null,
    actorName: dto.actor_name,
    actorRole: dto.actor_role,
    baseAction: dto.base_action,
    action: dto.action,
    segments: toSegments(dto.segments),
    createdAt: dto.created_at,
  };
}

export function mapHistoryFilterOptions(dto: HistoryFiltersDto): HistoryFilterOptions {
  return {
    participants: dto.participants.map((participant) => ({
      id: participant.id,
      name: participant.name,
      email: participant.email,
      firstName: participant.first_name,
      isOwner: participant.is_owner,
      role: participant.role,
    })),
    objects: dto.objects.map((object_) => ({
      id: object_.id,
      name: object_.name,
      address: object_.address,
      photoUrl: object_.photo_url,
    })),
  };
}
