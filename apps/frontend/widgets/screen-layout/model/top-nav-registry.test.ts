import { describe, expect, it } from 'vitest';
import { matchTopNavRoute } from './top-nav-registry';

/** Реестр «маршрут → статическая конфигурация шапки» (issue #1592):
 * матчер паттернов и статика трёх маршрутов прототипа. Элементы React
 * инспектируются как деревья без рендера — канон node-тестов
 * (CODING_STANDARDS, Testing). */
describe('matchTopNavRoute', () => {
  it('хаб «Платежи»: точный статический маршрут — крылья и компакт-конфиг', () => {
    const config = matchTopNavRoute('/payments');
    expect(config).toBeDefined();
    expect(config?.mobileWings).toBe(true);
    expect(config?.collapse?.title).toBe('Платежи');
    expect(config?.collapse?.search).toStrictEqual({
      href: '/payments/search',
      label: 'Найти платеж',
    });
  });

  it('подэкраны хаба не наследуют запись хаба (число сегментов)', () => {
    expect(matchTopNavRoute('/payments/search')).toBeUndefined();
    expect(matchTopNavRoute('/payments/objects')).toBeUndefined();
  });

  it('страница платежа: динамические сегменты дают fallback «Назад» и тайтл-плейсхолдер', () => {
    const config = matchTopNavRoute(
      '/properties/33333333-3333-4333-8333-333333333333/payments/55555555-5555-4555-8555-555555555551',
    );
    expect(config).toBeDefined();
    const leading = config?.leading;
    expect(leading).toBeDefined();
    // Канон «компонент как функция»: дерево элемента без рендера.
    expect(
      leading && typeof leading === 'object' && 'props' in leading
        ? (leading.props as { fallbackHref?: string }).fallbackHref
        : undefined,
    ).toBe('/properties/33333333-3333-4333-8333-333333333333/payments');
    const children = config?.children;
    expect(
      children && typeof children === 'object' && 'props' in children
        ? (children.props as { title?: unknown }).title
        : undefined,
    ).toBe('Платеж');
  });

  it('сегменты под страницей платежа не совпадают (график/история — свои записи тикета)', () => {
    expect(
      matchTopNavRoute(
        '/properties/33333333-3333-4333-8333-333333333333/payments/55555555-5555-4555-8555-555555555551/schedule',
      ),
    ).toBeUndefined();
  });

  it('обрезанный динамический маршрут не совпадает', () => {
    expect(matchTopNavRoute('/properties/33333333-3333-4333-8333-333333333333/payments')).toBeUndefined();
    expect(matchTopNavRoute('/properties')).toBeUndefined();
  });

  it('поиск операций: вариант search', () => {
    const config = matchTopNavRoute('/operations/search');
    expect(config).toStrictEqual({ variant: 'search' });
  });

  it('прочие маршруты без записи — undefined (шим после гидратации решает)', () => {
    expect(matchTopNavRoute('/')).toBeUndefined();
    expect(matchTopNavRoute('/operations')).toBeUndefined();
    expect(matchTopNavRoute('/history')).toBeUndefined();
  });
});
