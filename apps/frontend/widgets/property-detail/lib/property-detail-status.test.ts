import { describe, expect, it } from 'vitest';
import {
  buildPropertyKebabItems,
  buildPropertyManageActions,
  buildPropertyStatusSheetItems,
  deleteBlockedByRental,
  guardedStatusAction,
  propertyStatusSubtitle,
  type PropertyDetailActionKey,
} from './property-detail-status';

describe('propertyStatusSubtitle (подзаголовок шапки детали)', () => {
  it('активный — без подзаголовка', () => {
    expect(propertyStatusSubtitle('active')).toBeNull();
  });

  it('на ремонте — «На ремонте»', () => {
    expect(propertyStatusSubtitle('maintenance')).toBe('На ремонте');
  });

  it('в архиве — «В архиве»', () => {
    expect(propertyStatusSubtitle('archived')).toBe('В архиве');
  });
});

describe('buildPropertyKebabItems (кебаб-меню детали)', () => {
  it('активный объект: об объекте, изменить статус, совместный доступ', () => {
    expect(buildPropertyKebabItems('active', true)).toEqual([
      { key: 'about', label: 'Об объекте', danger: false },
      { key: 'change-status', label: 'Изменить статус', danger: false },
      { key: 'access', label: 'Совместный доступ', danger: false },
    ]);
  });

  it('объект на ремонте: тот же набор', () => {
    expect(buildPropertyKebabItems('maintenance', true).map((item) => item.key)).toEqual([
      'about',
      'change-status',
      'access',
    ]);
  });

  it('архивный: об объекте, вернуть из архива, совместный доступ', () => {
    expect(buildPropertyKebabItems('archived', true).map((item) => item.key)).toEqual([
      'about',
      'unarchive',
      'access',
    ]);
  });

  it('смотрящему: без статусных мутаций', () => {
    expect(buildPropertyKebabItems('active', false).map((item) => item.key)).toEqual([
      'about',
      'access',
    ]);
    expect(buildPropertyKebabItems('archived', false).map((item) => item.key)).toEqual([
      'about',
      'access',
    ]);
  });
});

describe('buildPropertyStatusSheetItems (шит смены статуса)', () => {
  it('владелец, без аренды: начать аренду, на ремонте, в архив', () => {
    expect(buildPropertyStatusSheetItems('active', false, true)).toEqual([
      { key: 'start-rental', label: 'Начать аренду', danger: false },
      { key: 'start-maintenance', label: 'Объект на ремонте', danger: false },
      { key: 'archive', label: 'Перевести в архив', danger: false },
    ]);
  });

  it('владелец, с незавершённой арендой: завершить аренду вместо начала', () => {
    expect(
      buildPropertyStatusSheetItems('active', true, true).map((item) => item.key),
    ).toEqual(['complete-rental', 'start-maintenance', 'archive']);
  });

  it('владелец, на ремонте: завершить ремонт, в архив', () => {
    expect(
      buildPropertyStatusSheetItems('maintenance', false, true).map((item) => item.key),
    ).toEqual(['finish-maintenance', 'archive']);
  });

  it('архивный — шит не открывается, набор не строится', () => {
    expect(buildPropertyStatusSheetItems('archived', false, true)).toEqual([]);
  });

  it('участник (#757 приёмка: нет мёртвых кнопок): без «Перевести в архив»', () => {
    expect(
      buildPropertyStatusSheetItems('active', false, false).map((item) => item.key),
    ).toEqual(['start-rental', 'start-maintenance']);
    expect(
      buildPropertyStatusSheetItems('active', true, false).map((item) => item.key),
    ).toEqual(['complete-rental', 'start-maintenance']);
    expect(
      buildPropertyStatusSheetItems('maintenance', false, false).map((item) => item.key),
    ).toEqual(['finish-maintenance']);
  });
});

type ManageInput = Parameters<typeof buildPropertyManageActions>[0];

const baseManage: ManageInput = {
  status: 'active',
  hasRental: false,
  isPinned: false,
  canMutate: true,
  canPin: true,
  canLeave: false,
  canLifecycle: true,
};

describe('buildPropertyManageActions (секция «Управление»)', () => {
  it('активный без аренды: полный список по макету', () => {
    expect(buildPropertyManageActions(baseManage).map((item) => item.key)).toEqual([
      'edit',
      'pin',
      'start-rental',
      'start-maintenance',
      'archive',
      'access',
      'delete',
    ]);
  });

  it('с арендой: «Завершить аренду» вместо «Начать аренду»', () => {
    const keys = buildPropertyManageActions({ ...baseManage, hasRental: true }).map(
      (item) => item.key,
    );
    expect(keys).toContain('complete-rental');
    expect(keys).not.toContain('start-rental');
  });

  it('основной объект: «Убрать из основных» вместо «Сделать основным»', () => {
    const keys = buildPropertyManageActions({ ...baseManage, isPinned: true }).map(
      (item) => item.key,
    );
    expect(keys).toContain('unpin');
    expect(keys).not.toContain('pin');
  });

  it('на ремонте: «Завершить ремонт» вместо «На ремонте»', () => {
    const keys = buildPropertyManageActions({ ...baseManage, status: 'maintenance' }).map(
      (item) => item.key,
    );
    expect(keys).toContain('finish-maintenance');
    expect(keys).not.toContain('start-maintenance');
  });

  it('архивный: вернуть из архива, доступ, удаление; правок нет', () => {
    const items = buildPropertyManageActions({ ...baseManage, status: 'archived' });
    expect(items.map((item) => item.key)).toEqual(['unarchive', 'access', 'delete']);
  });

  it('смотрящий: об объекте, совместный доступ, покинуть объект (#703)', () => {
    const items = buildPropertyManageActions({ ...baseManage, canMutate: false, canLeave: true });
    expect(items.map((item) => item.key)).toEqual(['about', 'access', 'leave']);
    expect(items.find((item) => item.key === 'leave')?.danger).toBe(true);
  });

  it('смотрящий без контекста доступа: без строки выхода (страховка)', () => {
    const items = buildPropertyManageActions({ ...baseManage, canMutate: false, canLeave: false });
    expect(items.map((item) => item.key)).toEqual(['about', 'access']);
  });

  it('архивный смотрящий: тот же набор — выход из архива не гасит (#703)', () => {
    const items = buildPropertyManageActions({
      ...baseManage,
      status: 'archived',
      canMutate: false,
      canLeave: true,
    });
    expect(items.map((item) => item.key)).toEqual(['about', 'access', 'leave']);
  });

  it('полный доступ — участник: «Покинуть объект» рядом с строками владельца (#703)', () => {
    const keys = buildPropertyManageActions({ ...baseManage, canLeave: true }).map(
      (item) => item.key,
    );
    expect(keys).toContain('leave');
    expect(keys.indexOf('leave')).toBe(keys.indexOf('access') + 1);
    expect(keys[keys.length - 1]).toBe('delete');
  });

  it('архивный полный доступ: выход между доступом и удалением (#703)', () => {
    const items = buildPropertyManageActions({
      ...baseManage,
      status: 'archived',
      canLeave: true,
    });
    expect(items.map((item) => item.key)).toEqual([
      'unarchive',
      'access',
      'leave',
      'delete',
    ]);
  });

  it('участник (#757 приёмка): без архива и удаления — нет мёртвых кнопок', () => {
    const keys = buildPropertyManageActions({ ...baseManage, canLifecycle: false, canLeave: true }).map(
      (item) => item.key,
    );
    expect(keys).toEqual(['edit', 'pin', 'start-rental', 'start-maintenance', 'access', 'leave']);
  });

  it('участник без права выхода: тот же набор без выхода', () => {
    const keys = buildPropertyManageActions({ ...baseManage, canLifecycle: false }).map(
      (item) => item.key,
    );
    expect(keys).toEqual(['edit', 'pin', 'start-rental', 'start-maintenance', 'access']);
  });

  it('участник, архивный объект: только доступ и выход — возврат/удаление владельческие', () => {
    const items = buildPropertyManageActions({
      ...baseManage,
      status: 'archived',
      canLifecycle: false,
      canLeave: true,
    });
    expect(items.map((item) => item.key)).toEqual(['access', 'leave']);
  });

  it('базовый тариф: строк основного объекта нет', () => {
    const keys = buildPropertyManageActions({ ...baseManage, canPin: false }).map(
      (item) => item.key,
    );
    expect(keys).not.toContain('pin');
    expect(keys).not.toContain('unpin');
  });

  it('удаление помечено danger', () => {
    const items = buildPropertyManageActions(baseManage);
    expect(items.find((item) => item.key === 'delete')?.danger).toBe(true);
    expect(items.find((item) => item.key === 'edit')?.danger).toBe(false);
  });
});

describe('подписи действий (канон домена)', () => {
  const labels = new Map(
    buildPropertyManageActions(baseManage).map((item) => [item.key, item.label]),
  );

  it.each([
    ['edit', 'Редактировать объект'],
    ['pin', 'Сделать основным'],
    ['start-rental', 'Начать аренду'],
    ['start-maintenance', 'Объект на ремонте'],
    ['archive', 'Перевести в архив'],
    ['access', 'Совместный доступ'],
    ['delete', 'Удалить объект'],
  ] as ReadonlyArray<readonly [PropertyDetailActionKey, string]>)(
    '%s — «%s»',
    (key, label) => {
      expect(labels.get(key)).toBe(label);
    },
  );

  it.each([
    [{ ...baseManage, hasRental: true }, 'complete-rental', 'Завершить аренду'],
    [{ ...baseManage, isPinned: true }, 'unpin', 'Убрать из основных'],
    [{ ...baseManage, status: 'maintenance' }, 'finish-maintenance', 'Завершить ремонт'],
    [{ ...baseManage, status: 'archived' }, 'unarchive', 'Вернуть из архива'],
  ] as ReadonlyArray<readonly [ManageInput, PropertyDetailActionKey, string]>)(
    'контекстный вариант %s — «%s»',
    (input, key, label) => {
      const items = buildPropertyManageActions(input);
      expect(items.find((item) => item.key === key)?.label).toBe(label);
    },
  );
});

describe('guardedStatusAction (гард смены статуса, #628)', () => {
  it('с арендой возвращает блокируемое действие для narrowing', () => {
    expect(guardedStatusAction('start-maintenance', true)).toBe('start-maintenance');
    expect(guardedStatusAction('archive', true)).toBe('archive');
  });

  it('с арендой остальные действия не гардит', () => {
    expect(guardedStatusAction('finish-maintenance', true)).toBeNull();
    expect(guardedStatusAction('complete-rental', true)).toBeNull();
    expect(guardedStatusAction('edit', true)).toBeNull();
    expect(guardedStatusAction('delete', true)).toBeNull();
    expect(guardedStatusAction('unarchive', true)).toBeNull();
  });

  it('без аренды не гардит ничего', () => {
    expect(guardedStatusAction('start-maintenance', false)).toBeNull();
    expect(guardedStatusAction('archive', false)).toBeNull();
  });
});

describe('deleteBlockedByRental (гард удаления, #632)', () => {
  it.each([
    ['delete', true, true, 'у арендованного перехватывает тап удаления'],
    ['archive', true, false, 'у арендованного остальные действия свободны'],
    ['complete-rental', true, false, 'гард статуса не дублируется'],
    ['edit', true, false, 'редактирование не гарлится'],
    ['delete', false, false, 'у свободного объекта удаление проходит'],
  ] as ReadonlyArray<readonly [PropertyDetailActionKey, boolean, boolean, string]>)(
    '%s при hasRental=%s — %s',
    (key, hasRental, blocked) => {
      expect(deleteBlockedByRental(key, hasRental)).toBe(blocked);
    },
  );
});
