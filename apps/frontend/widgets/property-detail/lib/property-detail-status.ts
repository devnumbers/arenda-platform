import type { PropertyStatus } from '@/entities/property';

/**
 * Чистая логика детали объекта по статусам (карта #583, тикет #588):
 * подзаголовок шапки, состав кебаба, шита смены статуса и секции
 * «Управление». Источники — Figma 1186:44996 (кебаб), 1554:100371 /
 * 1581:53679 (шиты), набор секций «Управление» 1554:98469; вариант
 * «Завершить аренду» для объекта с незавершённой арендой — резолюция
 * тикета (свитч Начать ↔ Завершить), уточняется на приёмке.
 */

export type PropertyDetailActionKey =
  | 'about'
  | 'change-status'
  | 'edit'
  | 'pin'
  | 'unpin'
  | 'start-rental'
  | 'complete-rental'
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

/** Подзаголовок в шапке под «Объект» (Figma 1581:53679, 1581:55564). */
export function propertyStatusSubtitle(status: PropertyStatus): string | null {
  if (status === 'maintenance') return 'На ремонте';
  if (status === 'archived') return 'В архиве';
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
 * на ремонте — 1581:53679. У активного с арендой первый пункт —
 * «Завершить аренду» (свитч, уточняется на приёмке). Архивному шита нет —
 * в кебабе прямой пункт «Вернуть из архива».
 */
export function buildPropertyStatusSheetItems(
  status: PropertyStatus,
  hasRental: boolean,
): ReadonlyArray<PropertyDetailAction> {
  if (status === 'archived') return [];
  if (status === 'maintenance') {
    return [
      action('finish-maintenance', 'Завершить ремонт', false),
      action('archive', 'Перевести в архив', false),
    ];
  }
  return [
    hasRental
      ? action('complete-rental', 'Завершить аренду', false)
      : action('start-rental', 'Начать аренду', false),
    action('start-maintenance', 'Объект на ремонте', false),
    action('archive', 'Перевести в архив', false),
  ];
}

/**
 * Guard смены статуса (#628, Figma 1583:55882): у объекта с незавершённой
 * арендой «Объект на ремонте» и «Перевести в архив» (из шита статуса и
 * из секции «Управление») не исполняются сразу — открывается guard-шит
 * «Нельзя изменить статус, пока объект арендован». Его «Завершить» —
 * составное действие (решение владельца 12.09, против двухшаговой
 * аннотации макета): завершает аренду (#627, сегодняшней датой) и тут же
 * применяет выбранный статус. Пункты в списках остаются (гард
 * перехватывает тап, не прячет). Возвращает само действие для narrowing
 * или null — мимо гарда. Чисто фронтовый: бэк переход
 * active→maintenance разрешает всегда.
 */
export type GuardedStatusAction = 'start-maintenance' | 'archive';

export function guardedStatusAction(
  key: PropertyDetailActionKey,
  hasRental: boolean,
): GuardedStatusAction | null {
  if (!hasRental) return null;
  if (key === 'start-maintenance' || key === 'archive') return key;
  return null;
}

/**
 * Гард удаления (#632): у арендованного «Удалить объект» не открывает шит
 * удаления — сперва «Завершить аренду» (#627), шит-подтверждение уже есть.
 * Чисто фронтовый перехват тапа: бэк прикрывает тем же правилом —
 * 409 property_occupied в DeleteProperty.
 */
export function deleteBlockedByRental(
  key: PropertyDetailActionKey,
  hasRental: boolean,
): boolean {
  return key === 'delete' && hasRental;
}

export type PropertyManageInput = {
  readonly status: PropertyStatus;
  /** Незавершённая аренда есть (occupancy списка, резолюция #584). */
  readonly hasRental: boolean;
  readonly isPinned: boolean;
  /** Мутационный доступ (canMutateProperty: роль и не-архив). */
  readonly canMutate: boolean;
  /** «Основной объект» доступен тарифу (платный, канон резолюции #584). */
  readonly canPin: boolean;
};

/** Секция «Управление» (Figma 1554:98469, активный): контекстный список
 * действий по статусу, аренде и пину. Без «Экспортировать объект» —
 * решение владельца 10.09. Архивный владелец (read-only, ADR 0028):
 * вернуть из архива, совместный доступ, удаление. Смотрящий (макет
 * 2235-100370, тикет #703): об объекте, совместный доступ, покинуть
 * объект — без «Сделать основным» из макета (мутация чужого объекта;
 * расхождение макета с тикетом — вопрос приёмки). `canMutate` здесь
 * РОЛЕВОЙ (владелец/участник против смотрящего) — в отличие от
 * propertyPermissions().canEdit, архив его не гасит. */
export function buildPropertyManageActions(
  input: PropertyManageInput,
): ReadonlyArray<PropertyDetailAction> {
  const { status, hasRental, isPinned, canMutate, canPin } = input;

  if (!canMutate) {
    return [
      action('about', 'Об объекте', false),
      action('access', 'Совместный доступ', false),
      action('leave', 'Покинуть объект', true),
    ];
  }

  if (status === 'archived') {
    return [
      action('unarchive', 'Вернуть из архива', false),
      action('access', 'Совместный доступ', false),
      action('delete', 'Удалить объект', true),
    ];
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
    hasRental
      ? action('complete-rental', 'Завершить аренду', false)
      : action('start-rental', 'Начать аренду', false),
  );
  items.push(
    status === 'maintenance'
      ? action('finish-maintenance', 'Завершить ремонт', false)
      : action('start-maintenance', 'Объект на ремонте', false),
    action('archive', 'Перевести в архив', false),
    action('access', 'Совместный доступ', false),
    action('delete', 'Удалить объект', true),
  );
  return items;
}

function action(
  key: PropertyDetailActionKey,
  label: string,
  danger: boolean,
): PropertyDetailAction {
  return { key, label, danger };
}
