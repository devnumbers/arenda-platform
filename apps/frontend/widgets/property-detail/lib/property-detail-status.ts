import type { PropertyStatus } from '@/entities/property';
import type { RentalActionState } from '@/features/rentals';

/**
 * Чистая логика детали объекта по статусам (карта #583, тикет #588):
 * подзаголовок шапки, состав кебаба, шита смены статуса и секции
 * «Управление». Источники — Figma 1186:44996 (кебаб), 1554:100371 /
 * 1581:53679 (шиты), набор секций «Управление» 1554:98469. Арендная
 * строка — состояние-машина «Действий аренды» (карта #984, тикет #986,
 * rentals/CONTEXT.md): аренды нет → «Начать аренду», «Ожидает начала» →
 * «Удалить аренду», «идёт» → «Завершить аренду»; машина одна — доменная,
 * surfaces синхронны (правка аренды — только со страницы аренды).
 */

export type PropertyDetailActionKey =
  | 'about'
  | 'change-status'
  | 'edit'
  | 'pin'
  | 'unpin'
  | 'start-rental'
  | 'complete-rental'
  | 'delete-rental'
  | 'start-maintenance'
  | 'finish-maintenance'
  | 'archive'
  | 'unarchive'
  | 'access'
  | 'leave'
  | 'delete';

export type PropertyDetailAction = {
  readonly key: PropertyDetailActionKey;
  readonly label: string;
  readonly danger: boolean;
};

/** Подзаголовок в шапке под «Объект» (Figma 1581:53679, 1581:55564).
 * Архив в шапке не подписывается: факт архива несёт пилюля «В архиве»
 * под адресом (#773, propertyHeaderPills) — иконка + видна любому
 * читателю, второй канал дублировал бы её. */
export function propertyStatusSubtitle(status: PropertyStatus): string | null {
  if (status === 'maintenance') return 'На ремонте';
  return null;
}

/** Кебаб (Figma 1186:44996 — активный, 1581:52407 — архивный). Смотрящему
 * (canMutate=false) статусные мутирующие пункты недоступны — остаются
 * чтение и совместный доступ. */
export function buildPropertyKebabItems(
  status: PropertyStatus,
  canMutate: boolean,
): ReadonlyArray<PropertyDetailAction> {
  if (status === 'archived') {
    return canMutate
      ? [
          action('about', 'Об объекте', false),
          action('unarchive', 'Вернуть из архива', false),
          action('access', 'Совместный доступ', false),
        ]
      : [action('about', 'Об объекте', false), action('access', 'Совместный доступ', false)];
  }
  return canMutate
    ? [
        action('about', 'Об объекте', false),
        action('change-status', 'Изменить статус', false),
        action('access', 'Совместный доступ', false),
      ]
    : [action('about', 'Об объекте', false), action('access', 'Совместный доступ', false)];
}

/**
 * Шит смены статуса (канон Modal-шита): без аренды — Figma 1554:100371,
 * на ремонте — 1581:53679. Арендная строка — по машине «Действий аренды»
 * (#986): «Ожидает начала» несёт «Удалить аренду» (danger: удаляет аренду
 * вместе с платём; подтверждение — канон удаления завершённой аренды).
 * «Перевести в архив» рисуется только владельцу (canLifecycle — зеркало
 * sharedpolicy.CanLifecycle): участнику сервер отвечает 403, мёртвых
 * кнопок быть не должно (приёмка #757). Архивному шита нет — в кебабе
 * прямой пункт «Вернуть из архива».
 */
export function buildPropertyStatusSheetItems(
  status: PropertyStatus,
  rentalState: RentalActionState,
  canLifecycle: boolean,
): ReadonlyArray<PropertyDetailAction> {
  if (status === 'archived') return [];
  if (status === 'maintenance') {
    const items = [action('finish-maintenance', 'Завершить ремонт', false)];
    if (canLifecycle) items.push(action('archive', 'Перевести в архив', false));
    return items;
  }
  const items = [
    rentalAction(rentalState),
    action('start-maintenance', 'Объект на ремонте', false),
  ];
  if (canLifecycle) items.push(action('archive', 'Перевести в архив', false));
  return items;
}

/** Арендная строка поверхностей объекта — срез машины «Действий аренды»:
 * none → Начало, upcoming → Удаление, идёт → Завершение. */
function rentalAction(rentalState: RentalActionState): PropertyDetailAction {
  switch (rentalState.kind) {
    case 'none':
      return action('start-rental', 'Начать аренду', false);
    case 'upcoming':
      return action('delete-rental', 'Удалить аренду', true);
    case 'active':
      return action('complete-rental', 'Завершить аренду', false);
  }
}

/**
 * Guard смены статуса (#628, Figma 1583:55882): у объекта с незавершённой
 * арендой — включая «Ожидает начала» (#986) — «Объект на ремонте» и
 * «Перевести в архив» (из шита статуса и из секции «Управление») не
 * исполняются сразу — открывается guard-шит «Нельзя изменить статус…».
 * Его подтверждение — составное действие по машине: у идущей аренды
 * «Завершить» завершает аренду (#627, сегодняшней датой) и тут же
 * применяет выбранный статус (решение владельца 12.09); у «Ожидает
 * начала» — удаляет аренду и тут же применяет статус (решение владельца
 * 30.09: завершение будущей аренды не существует). Пункты в списках
 * остаются (гард перехватывает тап, не прячет). Возвращает само действие
 * для narrowing или null — мимо гарда. Чисто фронтовый: бэк переход
 * active→maintenance разрешает всегда.
 */
export type GuardedStatusAction = 'start-maintenance' | 'archive';

export function guardedStatusAction(
  key: PropertyDetailActionKey,
  rentalState: RentalActionState,
): GuardedStatusAction | null {
  if (rentalState.kind === 'none') return null;
  if (key === 'start-maintenance' || key === 'archive') return key;
  return null;
}

/**
 * Гард удаления (#632): у объекта с незавершённой арендой — включая
 * «Ожидает начала» — «Удалить объект» не открывает шит удаления: сперва
 * разводка по машине (#986) — «Завершить аренду» у идущей (#627),
 * «Удалить аренду» у «Ожидает начала». Чисто фронтовый перехват тапа:
 * бэк прикрывает тем же правилом — 409 property_occupied в DeleteProperty.
 */
export function deleteBlockedByRental(
  key: PropertyDetailActionKey,
  rentalState: RentalActionState,
): boolean {
  return key === 'delete' && rentalState.kind !== 'none';
}

export type PropertyManageInput = {
  readonly status: PropertyStatus;
  /** Состояние «Действий аренды» (#986): none → Начать, upcoming →
   * Удалить, идёт → Завершить. */
  readonly rentalState: RentalActionState;
  readonly isPinned: boolean;
  /** Мутационный доступ — ролевой canManageMembers (#703); архив его
   * не гасит, билдер ветвит архив сам. */
  readonly canMutate: boolean;
  /** «Основной объект» доступен тарифу (платный, канон резолюции #584). */
  readonly canPin: boolean;
  /** Участник чужого объекта (#703): строка «Покинуть объект». */
  readonly canLeave: boolean;
  /** Жизненный цикл — архив/возврат/удаление (canLifecycle, зеркало
   * sharedpolicy.CanLifecycle): только владелец. Участнику эти строки не
   * рисуются — сервер отвечает 403, а мёртвых кнопок быть не должно
   * (приёмка #757, решение владельца). */
  readonly canLifecycle: boolean;
};

/** Секция «Управление» (Figma 1554:98469, активный): контекстный список
 * действий по статусу, аренде и пину. Без «Экспортировать объект» —
 * решение владельца 10.09. Архив/удаление — только владельцу
 * (canLifecycle; участнику 403 — приёмка #757). Архивный владелец
 * (read-only, ADR 0028): вернуть из архива, совместный доступ, удаление.
 * Смотрящий (макет 2235-100370, тикет #703): об объекте, совместный
 * доступ, покинуть объект — без «Сделать основным» из макета (мутация
 * чужого объекта; расхождение макета с тикетом — вопрос приёмки).
 * `canMutate` здесь РОЛЕВОЙ (владелец/участник против смотрящего) — в
 * отличие от propertyPermissions().canEdit, архив его не гасит. */
export function buildPropertyManageActions(
  input: PropertyManageInput,
): ReadonlyArray<PropertyDetailAction> {
  const { status, rentalState, isPinned, canMutate, canPin, canLeave, canLifecycle } = input;

  if (!canMutate) {
    // Смотрящий без контекста доступа — страховка «только чтение»,
    // строка выхода не рисуется.
    return canLeave
      ? [
          action('about', 'Об объекте', false),
          action('access', 'Совместный доступ', false),
          action('leave', 'Покинуть объект', true),
        ]
      : [action('about', 'Об объекте', false), action('access', 'Совместный доступ', false)];
  }

  if (status === 'archived') {
    if (!canLifecycle) {
      // Архивный объект участника: возврат/удаление владельческие —
      // остаются чтение доступа и выход.
      return canLeave
        ? [
            action('access', 'Совместный доступ', false),
            action('leave', 'Покинуть объект', true),
          ]
        : [action('access', 'Совместный доступ', false)];
    }
    const archived: PropertyDetailAction[] = [
      action('unarchive', 'Вернуть из архива', false),
      action('access', 'Совместный доступ', false),
    ];
    if (canLeave) archived.push(action('leave', 'Покинуть объект', true));
    archived.push(action('delete', 'Удалить объект', true));
    return archived;
  }

  const items: PropertyDetailAction[] = [action('edit', 'Редактировать объект', false)];
  if (canPin) {
    items.push(
      isPinned
        ? action('unpin', 'Убрать из основных', false)
        : action('pin', 'Сделать основным', false),
    );
  }
  items.push(
    rentalAction(rentalState),
    status === 'maintenance'
      ? action('finish-maintenance', 'Завершить ремонт', false)
      : action('start-maintenance', 'Объект на ремонте', false),
  );
  if (canLifecycle) items.push(action('archive', 'Перевести в архив', false));
  items.push(action('access', 'Совместный доступ', false));
  if (canLeave) items.push(action('leave', 'Покинуть объект', true));
  if (canLifecycle) items.push(action('delete', 'Удалить объект', true));
  return items;
}

function action(
  key: PropertyDetailActionKey,
  label: string,
  danger: boolean,
): PropertyDetailAction {
  return { key, label, danger };
}
