import { describe, expect, it } from 'vitest';
import {
  resolvePropertyDetailEmptySet,
  resolvePropertySectionEmpty,
  type PropertyDetailSectionKey,
  type PropertyEmptySet,
} from './property-sections';
import type { PropertyStatus } from '@/entities/property';

describe('resolvePropertyDetailEmptySet (набор пустых состояний)', () => {
  it('единственный активный объект — приветственный', () => {
    expect(resolvePropertyDetailEmptySet(1)).toBe('welcome');
  });

  it('два и больше — обычный', () => {
    expect(resolvePropertyDetailEmptySet(2)).toBe('regular');
  });
});

describe('resolvePropertySectionEmpty (копирайт пустых секций)', () => {
  const cases: ReadonlyArray<readonly [
    PropertyDetailSectionKey,
    PropertyEmptySet,
    PropertyStatus,
    { title: string; description: string | null; ctaLabel: string | null },
  ]> = [
    // Приветственные (первый объект) — титул + онбординг-описание
    // (Figma 1554:98470/98481).
    [
      'rental',
      'welcome',
      'active',
      {
        title: 'Аренда не добавлена',
        description: 'Укажите арендную ставку и сроки действия договора',
        ctaLabel: 'Добавить',
      },
    ],
    [
      'payments',
      'welcome',
      'active',
      {
        title: 'Платежи не добавлены',
        description: 'Добавьте регулярные платежи: коммунальные услуги, кредит или взносы',
        ctaLabel: 'Добавить',
      },
    ],
    [
      'operations',
      'welcome',
      'active',
      {
        title: 'Операций еще не было',
        description: 'Здесь появятся записи об оплате, ремонте и других операциях',
        ctaLabel: null,
      },
    ],
    [
      'contacts',
      'welcome',
      'active',
      {
        title: 'Контакты не добавлены',
        description: 'Добавьте контакты арендаторов, мастеров и других специалистов',
        ctaLabel: 'Добавить',
      },
    ],
    [
      'tasks',
      'welcome',
      'active',
      {
        title: 'Задач нет',
        description: 'Добавьте задачу — напоминание, звонок или вызов мастера',
        ctaLabel: 'Добавить',
      },
    ],
    [
      'about',
      'welcome',
      'active',
      {
        title: 'Характеристики не добавлены',
        description: 'Укажите площадь, этаж и другие параметры объекта',
        ctaLabel: 'Добавить',
      },
    ],
    // Обычные (второй и далее) — только серая строка, без описаний
    // (решение владельца 11.09, Figma 1554:99550/99561).
    [
      'rental',
      'regular',
      'active',
      { title: 'Аренда не добавлена', description: null, ctaLabel: 'Добавить' },
    ],
    [
      'payments',
      'regular',
      'active',
      { title: 'Платежи не добавлены', description: null, ctaLabel: 'Добавить' },
    ],
    [
      'operations',
      'regular',
      'active',
      { title: 'Операций еще не было', description: null, ctaLabel: null },
    ],
    [
      'contacts',
      'regular',
      'active',
      { title: 'Контакты не добавлены', description: null, ctaLabel: 'Добавить' },
    ],
    [
      'tasks',
      'regular',
      'active',
      { title: 'Задач нет', description: null, ctaLabel: 'Добавить' },
    ],
    [
      'about',
      'regular',
      'active',
      { title: 'Характеристики не добавлены', description: null, ctaLabel: 'Добавить' },
    ],
    // Статусные состояния аренды — полная анатомия со статусным описанием
    // в любом наборе (решение владельца 11.09, Figma 1581:53679 / 1581:52407).
    [
      'rental',
      'regular',
      'maintenance',
      {
        title: 'Аренда не добавлена',
        description: 'Добавьте условия аренды и настройте платеж',
        ctaLabel: 'Добавить',
      },
    ],
    [
      'rental',
      'regular',
      'archived',
      {
        title: 'Аренда не добавлена',
        description: 'Добавьте условия аренды и настройте платеж',
        ctaLabel: 'Добавить',
      },
    ],
  ];

  it.each(cases)('%s / %s / %s', (key, set, status, expected) => {
    expect(resolvePropertySectionEmpty(key, set, status)).toEqual(expected);
  });

  it('у операций нет CTA в любом наборе', () => {
    expect(resolvePropertySectionEmpty('operations', 'regular', 'active').ctaLabel).toBeNull();
    expect(resolvePropertySectionEmpty('operations', 'welcome', 'active').ctaLabel).toBeNull();
  });
});
