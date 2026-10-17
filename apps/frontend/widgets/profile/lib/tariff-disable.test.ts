import { describe, expect, it } from 'vitest';
import type { Property } from '@/entities/property';
import {
  disableConfirmTitle,
  disableSuccessTitle,
  keepPropertySelection,
  tariffDisableObjectsCard,
} from './tariff-disable';

function property(id: string, overrides: Partial<Property> = {}): Property {
  return {
    id,
    name: `Объект ${id}`,
    type: 'apartment',
    address: 'Москва, ул. Тестовая, 1',
    attributes: {},
    status: 'active',
    created_at: '2026-01-01T00:00:00Z',
    pinned_at: null,
    ...overrides,
  };
}

function own(id: string, overrides: Partial<Property> = {}): Property {
  return property(id, { access: { role: 'owner' }, ...overrides });
}

function invited(id: string): Property {
  return property(id, { access: { role: 'full_access' } });
}

describe('tariffDisableObjectsCard', () => {
  it('more than one active property: mock copy with the picker sentence', () => {
    expect(tariffDisableObjectsCard(5)).toStrictEqual({
      title: 'У вас 5 объектов',
      description:
        'После отключения все объекты, кроме одного, перейдут в архив — данные сохранятся, но пользоваться ими будет нельзя. Приглашенные участники потеряют доступ к вашим объектам. Выберите объект, который останется активным',
      showPicker: true,
    });
  });

  it('two properties still offer the choice', () => {
    expect(tariffDisableObjectsCard(2).showPicker).toBe(true);
  });

  it('single property: no picker, copy without the archive and selection clauses', () => {
    expect(tariffDisableObjectsCard(1)).toStrictEqual({
      title: 'У вас 1 объект',
      description:
        'После отключения данные сохранятся. Приглашенные участники потеряют доступ к вашим объектам.',
      showPicker: false,
    });
  });

  it('zero properties: pluralized title, no picker', () => {
    const card = tariffDisableObjectsCard(0);
    expect(card.title).toBe('У вас 0 объектов');
    expect(card.showPicker).toBe(false);
  });

  it('pluralizes 2-4 properties', () => {
    expect(tariffDisableObjectsCard(2).title).toBe('У вас 2 объекта');
  });
});

describe('disableConfirmTitle', () => {
  it('names the tariff in the question', () => {
    expect(disableConfirmTitle('pro')).toBe('Уверены, что хотите отключить тариф Про?');
    expect(disableConfirmTitle('business')).toBe(
      'Уверены, что хотите отключить тариф Бизнес?',
    );
  });
});

describe('disableSuccessTitle', () => {
  it('names the disabled tariff', () => {
    expect(disableSuccessTitle('pro')).toBe('Тариф Про отключен');
    expect(disableSuccessTitle('business')).toBe('Тариф Бизнес отключен');
  });
});

// #1518: выбор сохраняемого объекта — единый источник истины экрана.
// Кандидаты — только СВОИ активные объекты (бэк валидирует keepPropertyId
// по владению, 409 invalid_keep_property на чужой); приглашённые — не
// сохраняемые: при истечении архиваются только свои, чужие ноги гасит
// recipient-slots без выбора. payload определён только когда своих > 1,
// и всегда — один из choices (тихий фолбэк на первый).
describe('keepPropertySelection', () => {
  it('keeps the chosen own property in the visible state and in the payload', () => {
    const selection = keepPropertySelection('p2', [
      own('p1'),
      own('p2'),
      own('p3'),
    ]);
    expect(selection.choices.map((p) => p.id)).toStrictEqual(['p1', 'p2', 'p3']);
    expect(selection.selectedId).toBe('p2');
    expect(selection.payloadKeepPropertyId).toBe('p2');
  });

  it('falls back to the first own property when nothing is chosen', () => {
    const selection = keepPropertySelection(undefined, [own('p1'), own('p2')]);
    expect(selection.selectedId).toBe('p1');
    expect(selection.payloadKeepPropertyId).toBe('p1');
  });

  it('single own property: no payload — the backend keeps it by itself', () => {
    const selection = keepPropertySelection(undefined, [own('p1')]);
    expect(selection.choices).toHaveLength(1);
    expect(selection.selectedId).toBe('p1');
    expect(selection.payloadKeepPropertyId).toBeUndefined();
  });

  it('no own properties: nothing selected, nothing sent', () => {
    const selection = keepPropertySelection(undefined, []);
    expect(selection.choices).toStrictEqual([]);
    expect(selection.selectedId).toBeUndefined();
    expect(selection.payloadKeepPropertyId).toBeUndefined();
  });

  it('invited properties are not candidates: payload counts own only', () => {
    // Приглашённый рядом с одним своим не открывает выбор и не уходит в запрос.
    const single = keepPropertySelection(undefined, [own('p1'), invited('s1')]);
    expect(single.choices.map((p) => p.id)).toStrictEqual(['p1']);
    expect(single.payloadKeepPropertyId).toBeUndefined();

    // Два своих + приглашённый: выбор среди своих, приглашённый в payload не попадает.
    const multi = keepPropertySelection('p2', [
      own('p1'),
      invited('s1'),
      own('p2'),
    ]);
    expect(multi.choices.map((p) => p.id)).toStrictEqual(['p1', 'p2']);
    expect(multi.payloadKeepPropertyId).toBe('p2');
  });

  it('a foreign chosen id cannot reach the payload: falls back to first own', () => {
    const selection = keepPropertySelection('s1', [own('p1'), own('p2'), invited('s1')]);
    expect(selection.selectedId).toBe('p1');
    expect(selection.payloadKeepPropertyId).toBe('p1');
  });

  it('own maintenance property stays a candidate — the backend accepts it', () => {
    const selection = keepPropertySelection('p2', [
      own('p1'),
      own('p2', { status: 'maintenance' }),
    ]);
    expect(selection.choices.map((p) => p.id)).toStrictEqual(['p1', 'p2']);
    expect(selection.payloadKeepPropertyId).toBe('p2');
  });

  it('archived own property is not a candidate', () => {
    const selection = keepPropertySelection(undefined, [
      own('p1', { status: 'archived' }),
      own('p2'),
    ]);
    expect(selection.choices.map((p) => p.id)).toStrictEqual(['p2']);
    expect(selection.payloadKeepPropertyId).toBeUndefined();
  });
});
