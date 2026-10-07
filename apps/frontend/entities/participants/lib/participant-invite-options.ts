import type { Participant, ParticipantPropertyLeg } from '../model/types';

/** Минимальный обрезок объекта читающего, достаточный ряду мультичека. */
export type InvitePropertyOption = {
  readonly id: string;
  readonly name: string;
  readonly address: string;
  readonly photoUrl: string | undefined;
};

/** Вход `availableInviteProperties`: Property из кэша /properties
 * (address обязателен в контракте, фото — опционально). */
export type InvitePropertySource = {
  readonly id: string;
  readonly name: string;
  readonly address: string;
  readonly photoUrl?: string | null;
};

/** Свойство читающего → опция мультичека (фото — аватар объекта, ADR 0065:
 * одна приватная картинка на сущность). */
export function toInvitePropertyOption(property: InvitePropertySource): InvitePropertyOption {
  return {
    id: property.id,
    name: property.name,
    address: property.address,
    photoUrl: property.photoUrl ?? undefined,
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
