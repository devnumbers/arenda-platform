import { describe, expect, it } from 'vitest';
import { parseEnumParam } from './parse-enum-param';

describe('parseEnumParam — разбор enum-значения query-параметра адреса (#785)', () => {
  const parse = (value: string | string[] | undefined) =>
    parseEnumParam(value, ['asc', 'desc'] as const, 'desc');

  it('известное строковое значение возвращает как есть', () => {
    expect(parse('asc')).toBe('asc');
    expect(parse('desc')).toBe('desc');
  });

  it('отсутствующее, пустое и неизвестное значения — дефолт', () => {
    expect(parse(undefined)).toBe('desc');
    expect(parse('')).toBe('desc');
    expect(parse('newest')).toBe('desc');
  });

  it('массивное значение (битый дубликат параметра) — дефолт', () => {
    expect(parse(['asc'])).toBe('desc');
    expect(parse(['asc', 'desc'])).toBe('desc');
  });
});
