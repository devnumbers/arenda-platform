import { describe, expect, it } from 'vitest';
import { applyUrlParamsPatch, buildUrlWithParams } from './url-params';

/**
 * Чистое ядро сборки и патча адреса (pre-merge #785): хук use-url-params —
 * тонкий адаптер над next/navigation (как useSearchQueryState),
 * поведенческие гарантии пинятся на ядре. Главная — delete-своих-ключей
 * перед set: ключ группы без значения в patch снимается из адреса, а не
 * остаётся протухшим (урок фикса 0c64fcc3).
 */

describe('applyUrlParamsPatch — патч поверх текущих параметров', () => {
  it('ставит значения патча', () => {
    const next = applyUrlParamsPatch(new URLSearchParams(), { sort: 'title' }, ['sort', 'order']);
    expect(next.get('sort')).toBe('title');
  });

  it('сохраняет чужие параметры (фильтр ленты #524 не затирается)', () => {
    const current = new URLSearchParams('property=p1');
    const next = applyUrlParamsPatch(current, { sort: 'title' }, ['sort', 'order']);
    expect(next.get('property')).toBe('p1');
    expect(next.get('sort')).toBe('title');
  });

  it('снимает собственный ключ, отсутствующий в patch (дефолт не пишется)', () => {
    const current = new URLSearchParams('sort=title&order=desc');
    const next = applyUrlParamsPatch(current, {}, ['sort', 'order']);
    expect(next.toString()).toBe('');
  });

  it('частичный патч снимает только отсутствующий собственный ключ', () => {
    const current = new URLSearchParams('sort=title&order=desc&property=p1');
    const next = applyUrlParamsPatch(current, { sort: 'title' }, ['sort', 'order']);
    expect(next.get('sort')).toBe('title');
    expect(next.get('order')).toBeNull();
    expect(next.get('property')).toBe('p1');
  });

  it('смена значения поверх существующего', () => {
    const current = new URLSearchParams('order=desc');
    const next = applyUrlParamsPatch(current, { order: 'asc' }, ['order']);
    expect(next.get('order')).toBe('asc');
  });

  it('пустой own — чистая надстройка без снятия', () => {
    const current = new URLSearchParams('q=x');
    const next = applyUrlParamsPatch(current, { q: 'y' }, []);
    expect(next.toString()).toBe('q=y');
  });
});

describe('buildUrlWithParams — адрес с query или без', () => {
  it('пустые параметры — голый pathname', () => {
    expect(buildUrlWithParams('/tasks', new URLSearchParams())).toBe('/tasks');
  });

  it('непустые — pathname?query', () => {
    expect(buildUrlWithParams('/tasks', new URLSearchParams('sort=title'))).toBe('/tasks?sort=title');
  });
});
