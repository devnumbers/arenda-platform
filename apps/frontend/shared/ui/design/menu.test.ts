import { describe, expect, it } from 'vitest';
import { menuContentBaseClasses, MenuItem } from './menu';

/** Контракт кебаб-меню по макету 3183-86541 (аменда владельца 22.10:
 * высота пункта 48, зазор 0): node без DOM — канон тестов фронта.
 * Карточка — статичная строка классов на уровне модуля; пункт — вызов
 * без рендера, как Button в button.test.ts. */

const itemClasses = String(MenuItem({ children: 'Прочитать все' }).props.className);

describe('Карточка MenuContent — макет 3183-86541', () => {
  it('зазор пунктов 0 и паддинг 2px со всех сторон — не прежние gap-0.5/py-3', () => {
    expect(menuContentBaseClasses).toContain('gap-0');
    expect(menuContentBaseClasses).toContain('p-0.5');
    expect(menuContentBaseClasses).not.toContain('gap-0.5');
    expect(menuContentBaseClasses).not.toContain('py-3');
  });

  it('тень 0 8px 12px rgba(0,0,0,0.12) — не прежняя 0 8px 24px', () => {
    expect(menuContentBaseClasses).toContain('shadow-[0_8px_12px_rgba(0,0,0,0.12)]');
    expect(menuContentBaseClasses).not.toContain('0_8px_24px');
  });

  it('радиус карточки 18 (решение владельца 22.10, в макете 3183-86541 пока 24) — концентричен пункту 16 + паддинг 2', () => {
    expect(menuContentBaseClasses).toContain('rounded-[18px]');
  });
});

describe('Пункт MenuItem — «Button White Small» 3183-86542', () => {
  it('высота 48 (min-h-12), px 16, межстрочный 16 — не прежние h-11/px-5', () => {
    expect(itemClasses).toContain('min-h-12');
    expect(itemClasses).toContain('px-4');
    expect(itemClasses).toContain('leading-4');
    expect(itemClasses).not.toContain('h-11');
    expect(itemClasses).not.toContain('px-5');
  });

  it('иконка-текст 6px, радиус 16, текст 14/500, hover/active-пара — без изменений', () => {
    expect(itemClasses).toContain('gap-1.5');
    expect(itemClasses).toContain('rounded-button');
    expect(itemClasses).toContain('text-sm');
    expect(itemClasses).toContain('font-medium');
    expect(itemClasses).toContain('data-[highlighted]:bg-surface-muted');
    expect(itemClasses).toContain('active:bg-surface-muted-hover');
  });
});
