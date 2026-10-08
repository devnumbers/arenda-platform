import { participantLegBadge, sortByRuText } from '@/entities/participants';
import type { Property, PropertyType } from '@/entities/property';
import type { SharedAccessRole } from '@/shared/model/access';
import { parseEnumParam } from '@/shared/lib/parse-enum-param';

/** Направление чипа «Название» — как сортировка «Ваших участников» (#697). */
import type { ParticipantSortOrder as ParticipantsPropertySortOrder } from '@/entities/participants';

export type { ParticipantsPropertySortOrder };

/** Дефолтное направление «Объектов пользователей» — «А→Я», в адресе
 * не живёт (конвенция состояния в адресе, #785). */
export const DEFAULT_PARTICIPANTS_PROPERTY_ORDER: ParticipantsPropertySortOrder = 'asc';

/** Разбор ?order= списка «Объекты пользователей»: неизвестное и
 * отсутствующее значения — дефолт «А→Я». */
export function parseParticipantsPropertyOrderParams(
  order: string | string[] | undefined,
): ParticipantsPropertySortOrder {
  return parseEnumParam(order, ['asc', 'desc'], DEFAULT_PARTICIPANTS_PROPERTY_ORDER);
}

/** Собственный параметр направления в адресе — знание этого модуля;
 * писатель (ParticipantsPropertiesScreen) импортирует отсюда. */
export const PARTICIPANTS_PROPERTY_ORDER_PARAMS = ['order'] as const;

/** Патч направления для адреса: дефолтные значения параметров не создают
 * (пустой query — голый pathname); пишется через useUrlParams с
 * own: PARTICIPANTS_PROPERTY_ORDER_PARAMS (#785). */
export function serializeParticipantsPropertyOrderToParams(
  order: ParticipantsPropertySortOrder,
): Record<string, string> {
  return order === DEFAULT_PARTICIPANTS_PROPERTY_ORDER ? {} : { order };
}

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
  /** Тип объекта — глиф-плейсхолдер аватара (карта #1217, #1244). */
  readonly type: PropertyType;
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
 * Ряды экрана «Объектов пользователей» (#701): строки GET /properties, где
 * читающий не владелец (список уже несёт и свои, и чужие объекты —
 * access.role отличает их; suspended-доступы скрыты сервером — приходят
 * отдельным suspended_shared под блюр-карточки, тикет #702). Порядок входа
 * сохраняется — сортировка отдельно.
 */
export function userPropertyRows(properties: ReadonlyArray<Property>): UserPropertyRow[] {
  return properties.filter(isSharedProperty).map((property) => ({
    id: property.id,
    title: property.name,
    address: property.address,
    photoUrl: property.photoUrl ?? undefined,
    type: property.type,
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
  return sortByRuText(rows, (row) => row.title, order);
}
