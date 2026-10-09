import type { components } from '@/shared/api/dto';
import type { Participant, ParticipantPropertyLeg } from '../model/types';

/** Минимальный обрезок объекта читающего, достаточный ряду мультичека. */
export type InvitePropertyOption = {
  readonly id: string;
  readonly name: string;
  readonly address: string;
  readonly photoUrl: string | undefined;
  /** Тип объекта — глиф-плейсхолдер аватара (карта #1217, #1244; словарь
   * DTO — структурно тот же PropertyType слоя entities/property). */
  readonly type: components['schemas']['PropertyType'];
};

/** Вход `availableInviteProperties`: Property из кэша /properties
 * (address обязателен в контракте, фото — опционально). */
export type InvitePropertySource = {
  readonly id: string;
  readonly name: string;
  readonly address: string;
  readonly photoUrl?: string | null;
  readonly type: components['schemas']['PropertyType'];
};

/** Свойство читающего → опция мультичека (фото — аватар объекта, ADR 0065:
 * одна приватная картинка на сущность). */
export function toInvitePropertyOption(property: InvitePropertySource): InvitePropertyOption {
  return {
    id: property.id,
    name: property.name,
    address: property.address,
    photoUrl: property.photoUrl ?? undefined,
    type: property.type,
  };
}

/**
 * Объекты, доступные для «Пригласить в объект» (страница участника #698,
 * макет 2010-131329): объекты читающего минус те, где у участника уже есть
 * нога — активная, suspended или pending (повторный грант даёт
 * skipped_duplicate, поэтому нога любого статуса исключает объект).
 */
export function availableInviteProperties(
  properties: ReadonlyArray<InvitePropertySource>,
  participant: Participant,
): InvitePropertyOption[] {
  const granted = new Set<string>(
    participant.properties.map((leg: ParticipantPropertyLeg) => leg.propertyId),
  );
  return properties
    .filter((property) => !granted.has(property.id))
    .map(toInvitePropertyOption);
}
