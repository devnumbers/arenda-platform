import { describe, expect, it } from 'vitest';
import {
  buildPropertyKebabItems,
  buildPropertyManageActions,
  buildPropertyStatusSheetItems,
  propertyStatusSubtitle,
  statusChangeBlockedByRental,
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
  it('без аренды: начать аренду, на ремонте, в архив', () => {
    expect(buildPropertyStatusSheetItems('active', false)).toEqual([
      { key: 'start-rental', label: 'Начать аренду', danger: false },
      { key: 'start-maintenance', label: 'Объект на ремонте', danger: false },
      { key: 'archive', label: 'Перевести в архив', danger: false },
    ]);
  });

  it('с незавершённой арендой: завершить аренду вместо начала', () => {
    expect(
      buildPropertyStatusSheetItems('active', true).map((item) => item.key),
    ).toEqual(['complete-rental', 'start-maintenance', 'archive']);
  });

  it('на ремонте: завершить ремонт, в архив', () => {
    expect(
      buildPropertyStatusSheetItems('maintenance', false).map((item) => item.key),
    ).toEqual(['finish-maintenance', 'archive']);
  });

  it('архивный — шит не открывается, набор не строится', () => {
    expect(buildPropertyStatusSheetItems('archived', false)).toEqual([]);
  });
});

type ManageInput = Parameters<typeof buildPropertyManageActions>[0];

const baseManage: ManageInput = {
  status: 'active',
  hasRental: false,
  isPinned: false,
  canMutate: true,
  canPin: true,
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

  it('смотрящий (нельзя мутировать): только совместный доступ', () => {
    const items = buildPropertyManageActions({ ...baseManage, canMutate: false });
    expect(items.map((item) => item.key)).toEqual(['access']);
  });

  it('архивный смотрящий: тоже только совместный доступ', () => {
    const items = buildPropertyManageActions({
      ...baseManage,
      status: 'archived',
      canMutate: false,
    });
    expect(items.map((item) => item.key)).toEqual(['access']);
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

describe('statusChangeBlockedByRental (гард смены статуса, #628)', () => {
  it('с арендой блокирует ремонт и архив', () => {
    expect(statusChangeBlockedByRental('start-maintenance', true)).toBe(true);
    expect(statusChangeBlockedByRental('archive', true)).toBe(true);
  });

  it('с арендой остальные действия проходит', () => {
    expect(statusChangeBlockedByRental('finish-maintenance', true)).toBe(false);
    expect(statusChangeBlockedByRental('complete-rental', true)).toBe(false);
    expect(statusChangeBlockedByRental('edit', true)).toBe(false);
    expect(statusChangeBlockedByRental('delete', true)).toBe(false);
    expect(statusChangeBlockedByRental('unarchive', true)).toBe(false);
  });

  it('без аренды не блокирует ничего', () => {
    expect(statusChangeBlockedByRental('start-maintenance', false)).toBe(false);
    expect(statusChangeBlockedByRental('archive', false)).toBe(false);
  });
});
