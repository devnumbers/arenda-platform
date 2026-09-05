import { describe, expect, it } from 'vitest';

import { readTasksFeedFilter, tasksFeedFilterParams } from './tasks-feed-filter';

const FIRST = '0d5c6e2a-9f0e-4b1a-8c3d-2f7a1b9e5d40';
const SECOND = '1a2b3c4d-5e6f-4a1b-8c3d-2f7a1b9e5d99';

const paramsOf = (record: Record<string, string>) => ({
  get: (name: string) => record[name] ?? null,
});

describe('readTasksFeedFilter', () => {
  it('пустой URL — без фильтра («Все объекты»)', () => {
    expect(readTasksFeedFilter(paramsOf({}))).toEqual({ propertyIds: [] });
  });

  it('читает один uuid — прежний формат ссылки', () => {
    expect(readTasksFeedFilter(paramsOf({ property: FIRST }))).toEqual({
      propertyIds: [FIRST],
    });
  });

  it('читает список через запятую (мультивыбор, #547)', () => {
    expect(readTasksFeedFilter(paramsOf({ property: `${FIRST},${SECOND}` }))).toEqual({
      propertyIds: [FIRST, SECOND],
    });
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
    expect(tasksFeedFilterParams({ propertyIds: [] })).toEqual({});
  });

  it('один объект — параметр property с одним id', () => {
    expect(tasksFeedFilterParams({ propertyIds: [FIRST] })).toEqual({ property: FIRST });
  });

  it('несколько объектов — список через запятую', () => {
    expect(tasksFeedFilterParams({ propertyIds: [FIRST, SECOND] })).toEqual({
      property: `${FIRST},${SECOND}`,
    });
  });

  it('чтение и запись симметричны', () => {
    const filter = { propertyIds: [FIRST, SECOND] };
    expect(readTasksFeedFilter(paramsOf(tasksFeedFilterParams(filter)))).toEqual(filter);
  });
});
