import type { Property } from '@/entities/property';

/**
 * Имена участников для ряда карточки объекта (карта #692, тикет #702;
 * Figma 2200-97368): ряд носят только СВОИ объекты — активные участники в
 * порядке приглашения, как отдал список. Чужие объекты (access.role ≠
 * owner) ряда не ведут: их поверхность — бейдж роли и (для подвесших)
 * блюр-карточка.
 */
export function cardParticipantNames(property: Property): readonly string[] {
  if (property.access !== undefined && property.access.role !== 'owner') {
    return [];
  }
  return property.member_names ?? [];
}
