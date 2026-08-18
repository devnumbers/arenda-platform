import type { components } from '@/shared/api/dto';
import type { FreeReminder, UpcomingFreeReminder } from './types';

type FreeReminderResponse = components['schemas']['FreeReminderResponse'];
type UpcomingFreeReminderResponse =
  components['schemas']['UpcomingFreeReminderResponse'];

export function mapFreeReminderResponse(dto: FreeReminderResponse): FreeReminder {
  return {
    id: dto.id,
    ownerId: dto.owner_id,
    propertyId: dto.property_id,
    title: dto.title,
    triggerAt: dto.trigger_at,
    periodicity: dto.periodicity,
    createdAt: dto.created_at,
    updatedAt: dto.updated_at,
  };
}

export function mapUpcomingFreeReminderResponse(
  dto: UpcomingFreeReminderResponse,
): UpcomingFreeReminder {
  return {
    freeReminderId: dto.free_reminder_id,
    title: dto.title,
    propertyId: dto.property_id,
    triggerAt: dto.trigger_at,
    periodicity: dto.periodicity,
  };
}
