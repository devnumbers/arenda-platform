import { describe, expect, it } from 'vitest';

import { readTasksFeedFilter, tasksFeedFilterParams } from './tasks-feed-filter';

const PROPERTY_ID = '0d5c6e2a-9f0e-4b1a-8c3d-2f7a1b9e5d40';

const paramsOf = (record: Record<string, string>) => ({
  get: (name: string) => record[name] ?? null,
});

describe('readTasksFeedFilter', () => {
  it('пустой URL — без фильтра («Все объекты»)', () => {
    expect(readTasksFeedFilter(paramsOf({}))).toEqual({ propertyId: null });
  });

  it('читает валидный uuid параметра property', () => {
    expect(readTasksFeedFilter(paramsOf({ property: PROPERTY_ID }))).toEqual({
      propertyId: PROPERTY_ID,
    });
  });

  it('мусор и пустая строка отбрасываются — фильтр не применяется', () => {
    expect(readTasksFeedFilter(paramsOf({ property: 'квартира' })).propertyId).toBeNull();
    expect(readTasksFeedFilter(paramsOf({ property: '' })).propertyId).toBeNull();
    expect(readTasksFeedFilter(paramsOf({ property: `${PROPERTY_ID}zz` })).propertyId).toBeNull();
  });

  it('uuid в верхнем регистре валиден', () => {
    expect(readTasksFeedFilter(paramsOf({ property: PROPERTY_ID.toUpperCase() })).propertyId).toBe(
      PROPERTY_ID.toUpperCase(),
    );
  });
});

describe('tasksFeedFilterParams', () => {
  it('без фильтра параметров нет', () => {
    expect(tasksFeedFilterParams({ propertyId: null })).toEqual({});
  });

  it('с фильтром — параметр property', () => {
    expect(tasksFeedFilterParams({ propertyId: PROPERTY_ID })).toEqual({
      property: PROPERTY_ID,
    });
  });

  it('чтение и запись симметричны', () => {
    const filter = { propertyId: PROPERTY_ID };
    expect(readTasksFeedFilter(paramsOf(tasksFeedFilterParams(filter)))).toEqual(filter);
  });
});
