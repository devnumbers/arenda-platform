import { describe, expect, it } from 'vitest';
import { parseStringParam } from './parse-string-param';

describe('parseStringParam — строка или битое', () => {
  it('строка возвращается как есть (даже пустая)', () => {
    expect(parseStringParam('role')).toBe('role');
    expect(parseStringParam('')).toBe('');
  });

  it('отсутствующий параметр — undefined', () => {
    expect(parseStringParam(undefined)).toBeUndefined();
  });

  it('массив (битый дубликат) — undefined, не первый элемент', () => {
    expect(parseStringParam(['a', 'b'])).toBeUndefined();
    expect(parseStringParam(['a'])).toBeUndefined();
  });
});
