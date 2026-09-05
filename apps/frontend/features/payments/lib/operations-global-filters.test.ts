import { describe, expect, it } from 'vitest';

import {
  globalOperationsFiltersParams,
  operationsPropertyChipLabel,
  readGlobalOperationsFilters,
  resolveGlobalFilterReturnPath,
} from './operations-global-filters';

const TODAY = '2026-09-02';

const paramsOf = (record: Record<string, string>) => ({
  get: (name: string) => record[name] ?? null,
});

const PROP_A = '0198b4a6-1c2d-7e3f-8a9b-0c1d2e3f4a5b';
const PROP_B = '0198b4a6-1c2d-7e3f-8a9b-0c1d2e3f4a5c';

describe('readGlobalOperationsFilters', () => {
  it('пустой URL — дефолт: текущий месяц, все объекты и категории', () => {
    expect(readGlobalOperationsFilters(paramsOf({}), TODAY)).toEqual({
      period: null,
      categories: [],
      propertyIds: [],
    });
  });

  it('читает период, категории и мультивыбор объектов', () => {
    expect(
      readGlobalOperationsFilters(
        paramsOf({
          from: '2026-08-10',
          to: '2026-09-01',
          category: 'rent, security',
          property: `${PROP_A}, ${PROP_B}`,
        }),
        TODAY,
      ),
    ).toEqual({
      period: { from: '2026-08-10', to: '2026-09-01' },
      categories: ['rent', 'security'],
      propertyIds: [PROP_A, PROP_B],
    });
  });

  it('битые uuid отбрасываются, дубли схлопываются, порядок живёт', () => {
    expect(
      readGlobalOperationsFilters(
        paramsOf({ property: `${PROP_A}, not-a-uuid, ${PROP_B}, ${PROP_A}` }),
        TODAY,
      ),
    ).toEqual({
      period: null,
      categories: [],
      propertyIds: [PROP_A, PROP_B],
    });
  });

  it('перевёрнутый период отбрасывается, как на объектном экране', () => {
    expect(
      readGlobalOperationsFilters(paramsOf({ from: '2026-09-10', to: '2026-09-01' }), TODAY),
    ).toEqual({ period: null, categories: [], propertyIds: [] });
  });
});

describe('globalOperationsFiltersParams', () => {
  it('дефолт даёт пустой набор параметров', () => {
    expect(
      globalOperationsFiltersParams({ period: null, categories: [], propertyIds: [] }),
    ).toEqual({});
  });

  it('явный выбор пишет from/to/category/property', () => {
    expect(
      globalOperationsFiltersParams({
        period: { from: '2026-08-01', to: '2026-08-31' },
        categories: ['rent'],
        propertyIds: [PROP_A, PROP_B],
      }),
    ).toEqual({
      from: '2026-08-01',
      to: '2026-08-31',
      category: 'rent',
      property: `${PROP_A},${PROP_B}`,
    });
  });
});

describe('operationsPropertyChipLabel', () => {
  it('без выбора — «Все объекты»', () => {
    expect(operationsPropertyChipLabel([])).toBe('Все объекты');
  });

  it('один объект — «1 объект»', () => {
    expect(operationsPropertyChipLabel([PROP_A])).toBe('1 объект');
  });

  it('несколько — счётчик со склонением', () => {
    expect(operationsPropertyChipLabel([PROP_A, PROP_B])).toBe('2 объекта');
    expect(operationsPropertyChipLabel([PROP_A, PROP_B, PROP_A, PROP_B, PROP_A])).toBe(
      '5 объектов',
    );
  });
});

describe('resolveGlobalFilterReturnPath', () => {
  it('без параметра — главный список операций', () => {
    expect(resolveGlobalFilterReturnPath(null)).toBe('/operations');
    expect(resolveGlobalFilterReturnPath('')).toBe('/operations');
  });

  it('маршруты зоны операций возвращаются как есть, с их query', () => {
    expect(resolveGlobalFilterReturnPath('/operations')).toBe('/operations');
    expect(
      resolveGlobalFilterReturnPath(
        `/operations?from=2026-08-01&to=2026-08-31&property=${PROP_A}`,
      ),
    ).toBe(`/operations?from=2026-08-01&to=2026-08-31&property=${PROP_A}`);
    expect(resolveGlobalFilterReturnPath('/operations/search?q=тест')).toBe(
      '/operations/search?q=тест',
    );
  });

  it('чужие и опасные пути отбрасываются — главный список', () => {
    expect(resolveGlobalFilterReturnPath('/properties/x/operations')).toBe('/operations');
    expect(resolveGlobalFilterReturnPath('/operations-archive')).toBe('/operations');
    expect(resolveGlobalFilterReturnPath('//evil.example')).toBe('/operations');
    expect(resolveGlobalFilterReturnPath('https://evil.example')).toBe('/operations');
    expect(resolveGlobalFilterReturnPath('/login')).toBe('/operations');
  });
});
