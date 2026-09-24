import { describe, expect, it } from 'vitest';
import { readCsvParam, readCsvUuidParam } from './parse-csv-uuid-param';

const UUID_A = '0198b4a6-1c2d-7e3f-8a9b-0c1d2e3f4a5b';
const UUID_B = '0198b4a6-1c2d-7e3f-8a9b-0c1d2e3f4a5c';

describe('readCsvParam', () => {
  type Slug = 'rent' | 'mortgage';
  const isSlug = (value: string): value is Slug => value === 'rent' || value === 'mortgage';

  it('без предиката: непустые элементы, пробелы сняты, дубли схлопнуты, порядок первого появления', () => {
    expect(readCsvParam(null)).toEqual([]);
    expect(readCsvParam('')).toEqual([]);
    expect(readCsvParam(' rent , ,rent,mortgage')).toEqual(['rent', 'mortgage']);
  });

  it('с предикатом: невалидные отбрасываются по одному, валидные остаются', () => {
    expect(readCsvParam('rent,чужое,mortgage,чужое', isSlug)).toEqual(['rent', 'mortgage']);
    expect(readCsvParam('чужое', isSlug)).toEqual([]);
    expect(readCsvParam(null, isSlug)).toEqual([]);
  });
});

describe('readCsvUuidParam', () => {
  it('отсутствующий параметр и пустое значение — пустой список', () => {
    expect(readCsvUuidParam(null)).toEqual([]);
    expect(readCsvUuidParam('')).toEqual([]);
    expect(readCsvUuidParam(',')).toEqual([]);
  });

  it('читает один uuid и список через запятую', () => {
    expect(readCsvUuidParam(UUID_A)).toEqual([UUID_A]);
    expect(readCsvUuidParam(`${UUID_A},${UUID_B}`)).toEqual([UUID_A, UUID_B]);
  });

  it('пробелы вокруг id снимаются, дубли схлопываются, порядок первого появления сохранён', () => {
    expect(readCsvUuidParam(` ${UUID_A} , ${UUID_B}, ${UUID_A} `)).toEqual([UUID_A, UUID_B]);
  });

  it('битые элементы отбрасываются по одному, валидные остаются', () => {
    expect(readCsvUuidParam(`квартира,${UUID_A},not-a-uuid,${UUID_B}`)).toEqual([UUID_A, UUID_B]);
    expect(readCsvUuidParam('квартира')).toEqual([]);
  });

  it('uuid в верхнем регистре валиден и сохраняет регистр', () => {
    expect(readCsvUuidParam(UUID_A.toUpperCase())).toEqual([UUID_A.toUpperCase()]);
  });
});
