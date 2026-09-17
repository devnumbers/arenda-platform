import { participantLegBadge } from '@/entities/participants';
import type { Property } from '@/entities/property';
import type { SharedAccessRole } from '@/shared/model/access';

/** Направление чипа «Название» — как сортировка «Ваших участников» (#697). */
export type ParticipantsPropertySortOrder = 'asc' | 'desc';

/** Модель бейджа ноги — канон participantLegBadge (#698). */
export type UserPropertyBadge = ReturnType<typeof participantLegBadge>;

/** Данные ряда «Объектов пользователей» (карта #692, тикет #701): всё,
 * что рисует строка, вычислено один раз (ViewModel) — экрану остаётся
 * только рендер. */
export type UserPropertyRow = {
  readonly id: string;
  readonly title: string;
  readonly address: string;
  readonly photoUrl: string | undefined;
  /** Роль доступа — статус-строке шита действий («Вам доступно
   * редактирование» / «Вам доступен просмотр»). */
  readonly role: SharedAccessRole;
  /** Бейдж роли доступа: full_access → «Редактирование», viewer →
   * «Просмотр» (канон participantLegBadge #698; suspended-ноги в списке
   * GET /properties скрыты — их показ, тикет #702). */
  readonly badge: UserPropertyBadge;
};

/** Чужой объект: строка GET /properties с контекстом доступа не-владельца. */
type SharedProperty = Property & {
  readonly access: { readonly role: SharedAccessRole };
};

function isSharedProperty(property: Property): property is SharedProperty {
  return property.access !== undefined && property.access.role !== 'owner';
}

/**
 * Ряды экрана «Объекты пользователей» (#701): строки GET /properties, где
 * читающий не владелец (список уже несёт и свои, и чужие объекты —
 * access.role отличает их; suspended-доступы скрыты сервером —
 * hidden_shared_count, тикет #702). Порядок входа сохраняется — сортировка
 * отдельно.
 */
export function userPropertyRows(properties: ReadonlyArray<Property>): UserPropertyRow[] {
  return properties.filter(isSharedProperty).map((property) => ({
    id: property.id,
    title: property.name,
    address: property.address,
    photoUrl: property.photos?.[0]?.url,
    role: property.access.role,
    badge: participantLegBadge({
      propertyId: property.id,
      title: property.name,
      role: property.access.role,
      status: 'active',
    }),
  }));
}

/** Чип «Название» (asc/desc): русская коллация по титулу, вход не
 * мутируется — прецедент sortParticipantsByName (#697). */
export function sortUserPropertyRows(
  rows: ReadonlyArray<UserPropertyRow>,
  order: ParticipantsPropertySortOrder,
): UserPropertyRow[] {
  const sorted = [...rows];
  const collator = new Intl.Collator('ru');
  sorted.sort((a, b) =>
    order === 'asc'
      ? collator.compare(a.title.toLowerCase(), b.title.toLowerCase())
      : collator.compare(b.title.toLowerCase(), a.title.toLowerCase()),
  );
  return sorted;
}
