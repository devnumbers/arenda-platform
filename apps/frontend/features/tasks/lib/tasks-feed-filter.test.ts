import { describe, expect, it } from 'vitest';

import {
  readTasksFeedFilter,
  tasksFeedFilterParams,
  tasksFeedPropertyParam,
  tasksFeedWithoutPropertyParam,
} from './tasks-feed-filter';

const FIRST = '0d5c6e2a-9f0e-4b1a-8c3d-2f7a1b9e5d40';
const SECOND = '1a2b3c4d-5e6f-4a1b-8c3d-2f7a1b9e5d99';

const paramsOf = (record: Record<string, string>) => ({
  get: (name: string) => record[name] ?? null,
});

const ALL = { propertyIds: [] as ReadonlyArray<string>, withoutProperty: false };

describe('readTasksFeedFilter', () => {
  it('пустой URL — без фильтра («Все задачи»)', () => {
    expect(readTasksFeedFilter(paramsOf({}))).toEqual(ALL);
  });

  it('читает один uuid — прежний формат ссылки', () => {
    expect(readTasksFeedFilter(paramsOf({ property: FIRST }))).toEqual({
      propertyIds: [FIRST],
      withoutProperty: false,
    });
  });

  it('читает список через запятую (мультивыбор, #547)', () => {
    expect(readTasksFeedFilter(paramsOf({ property: `${FIRST},${SECOND}` }))).toEqual({
      propertyIds: [FIRST, SECOND],
      withoutProperty: false,
    });
  });

  it('безProperty=1 — «Общие задачи» (решение владельца 2026-09-07)', () => {
    expect(readTasksFeedFilter(paramsOf({ [tasksFeedWithoutPropertyParam]: '1' }))).toEqual({
      propertyIds: [],
      withoutProperty: true,
    });
  });

  it('безProperty=true тоже читается, прочие значения — нет', () => {
    expect(
      readTasksFeedFilter(paramsOf({ [tasksFeedWithoutPropertyParam]: 'true' })).withoutProperty,
    ).toBe(true);
    expect(
      readTasksFeedFilter(paramsOf({ [tasksFeedWithoutPropertyParam]: '0' })).withoutProperty,
    ).toBe(false);
    expect(
      readTasksFeedFilter(paramsOf({ [tasksFeedWithoutPropertyParam]: 'мусор' })).withoutProperty,
    ).toBe(false);
  });

  it('объекты и «Общие задачи» читаются вместе (union)', () => {
    expect(
      readTasksFeedFilter(
        paramsOf({ property: FIRST, [tasksFeedWithoutPropertyParam]: '1' }),
      ),
    ).toEqual({ propertyIds: [FIRST], withoutProperty: true });
  });

  it('пробелы вокруг id игнорируются, дубли схлопываются', () => {
    expect(
      readTasksFeedFilter(paramsOf({ property: ` ${FIRST} , ${SECOND}, ${FIRST} ` })).propertyIds,
    ).toEqual([FIRST, SECOND]);
  });

  it('битые элементы отбрасываются, валидные остаются', () => {
    expect(
      readTasksFeedFilter(paramsOf({ property: `квартира,${FIRST},` })).propertyIds,
    ).toEqual([FIRST]);
  });

  it('одни битые элементы — фильтр не применяется', () => {
    expect(readTasksFeedFilter(paramsOf({ property: 'квартира,' })).propertyIds).toEqual([]);
    expect(readTasksFeedFilter(paramsOf({ property: '' })).propertyIds).toEqual([]);
  });

  it('uuid в верхнем регистре валиден', () => {
    expect(readTasksFeedFilter(paramsOf({ property: FIRST.toUpperCase() })).propertyIds).toEqual([
      FIRST.toUpperCase(),
    ]);
  });
});

describe('tasksFeedFilterParams', () => {
  it('без фильтра параметров нет', () => {
    expect(tasksFeedFilterParams(ALL)).toEqual({});
  });

  it('один объект — параметр property с одним id', () => {
    expect(tasksFeedFilterParams({ propertyIds: [FIRST], withoutProperty: false })).toEqual({
      property: FIRST,
    });
  });

  it('несколько объектов — список через запятую', () => {
    expect(
      tasksFeedFilterParams({ propertyIds: [FIRST, SECOND], withoutProperty: false }),
    ).toEqual({ property: `${FIRST},${SECOND}` });
  });

  it('«Общие задачи» — отдельный параметр без property', () => {
    expect(tasksFeedFilterParams({ propertyIds: [], withoutProperty: true })).toEqual({
      [tasksFeedWithoutPropertyParam]: '1',
    });
  });

  it('union — оба параметра', () => {
    expect(
      tasksFeedFilterParams({ propertyIds: [FIRST], withoutProperty: true }),
    ).toEqual({ property: FIRST, [tasksFeedWithoutPropertyParam]: '1' });
  });

  it('чтение и запись симметричны', () => {
    const filter = { propertyIds: [FIRST, SECOND], withoutProperty: true };
    expect(readTasksFeedFilter(paramsOf(tasksFeedFilterParams(filter)))).toEqual(filter);
  });

  it('имена параметров — стабильный контракт', () => {
    expect(tasksFeedPropertyParam).toBe('property');
    expect(tasksFeedWithoutPropertyParam).toBe('withoutProperty');
  });
});
