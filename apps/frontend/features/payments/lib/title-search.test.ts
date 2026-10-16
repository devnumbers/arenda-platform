import { describe, expect, it } from 'vitest';
import { matchesTitleSearch } from './title-search';

describe('matchesTitleSearch', () => {
  it('пустой запрос матчит всё', () => {
    expect(matchesTitleSearch('', 'Арендная плата')).toBe(true);
    expect(matchesTitleSearch('   ', 'Арендная плата')).toBe(true);
  });

  it('регистронезависимое вхождение подстроки', () => {
    expect(matchesTitleSearch('арен', 'Арендная плата')).toBe(true);
    expect(matchesTitleSearch('ПЛАТА', 'Арендная плата')).toBe(true);
    expect(matchesTitleSearch('коммун', 'Коммунальные услуги')).toBe(true);
  });

  it('нет вхождения — нет совпадения', () => {
    expect(matchesTitleSearch('коммун', 'Арендная плата')).toBe(false);
    expect(matchesTitleSearch('ипотека', '')).toBe(false);
  });

  it('запрос обрезается по краям', () => {
    expect(matchesTitleSearch('  арендная  ', 'Арендная плата')).toBe(true);
  });
});
