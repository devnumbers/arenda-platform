import { afterEach, describe, expect, it, vi } from 'vitest';
import { mergeTopNavConfig, topNavPatchStore } from './top-nav-registration';
import type { TopNavProps } from './top-nav-view';

/** Регистрационный стор и сборка effective-пропов персистентной шапки
 * (issue #1592): иммутабельные снапшоты для useSyncExternalStore,
 * shallow-merge «реестр ← шим», признак присутствия шапки. */
describe('topNavPatchStore', () => {
  const slots: string[] = [];

  afterEach(() => {
    for (const slotId of slots) topNavPatchStore.delete(slotId);
    slots.length = 0;
  });

  const register = (props: TopNavProps): string => {
    const slotId = `slot-${slots.length + 1}`;
    slots.push(slotId);
    topNavPatchStore.set(slotId, props);
    return slotId;
  };

  it('set кладёт слот в снапшот, снапшот иммутабелен (новая карта)', () => {
    const before = topNavPatchStore.getSnapshot();
    const listener = vi.fn();
    const unsubscribe = topNavPatchStore.subscribe(listener);
    const slotId = register({ children: 'Тайтл' });
    const after = topNavPatchStore.getSnapshot();
    expect(before).not.toBe(after);
    expect(after.get(slotId)).toStrictEqual({ children: 'Тайтл' });
    expect(before.has(slotId)).toBe(false);
    expect(listener).toHaveBeenCalled();
    unsubscribe();
  });

  it('delete убирает слот и уведомляет; чужой слот — no-op без уведомления', () => {
    const slotId = register({ variant: 'search' });
    const listener = vi.fn();
    const unsubscribe = topNavPatchStore.subscribe(listener);
    topNavPatchStore.delete('no-such-slot');
    expect(listener).not.toHaveBeenCalled();
    topNavPatchStore.delete(slotId);
    expect(topNavPatchStore.getSnapshot().has(slotId)).toBe(false);
    expect(listener).toHaveBeenCalled();
    unsubscribe();
  });

  it('серверный снапшот — стабильная ссылка между вызовами', () => {
    expect(topNavPatchStore.getServerSnapshot()).toBe(topNavPatchStore.getServerSnapshot());
  });
});

describe('mergeTopNavConfig', () => {
  it('без реестра и регистраций шапки нет', () => {
    expect(mergeTopNavConfig(undefined, new Map()).present).toBe(false);
  });

  it('реестр один даёт присутствие и пропы без изменений', () => {
    const base: TopNavProps = { mobileWings: true };
    const merged = mergeTopNavConfig(base, new Map());
    expect(merged.present).toBe(true);
    expect(merged.props).toBe(base);
  });

  it('регистрация только с реестром перебивает поле, не трогая остальные', () => {
    const base: TopNavProps = {
      mobileWings: true,
      collapse: { title: 'Платежи' },
    };
    const patches = new Map<string, TopNavProps>([
      ['slot-a', { collapse: { title: 'Платежи', trailing: 'плюс' } }],
    ]);
    const merged = mergeTopNavConfig(base, patches);
    expect(merged.present).toBe(true);
    expect(merged.props.mobileWings).toBe(true);
    expect(merged.props.collapse?.title).toBe('Платежи');
    expect(merged.props.collapse?.trailing).toBe('плюс');
  });

  it('поздний слот перебивает ранний; регистрации живут и без реестра', () => {
    const patches = new Map<string, TopNavProps>([
      ['slot-a', { children: 'старый' }],
      ['slot-b', { children: 'новый' }],
    ]);
    const merged = mergeTopNavConfig(undefined, patches);
    expect(merged.present).toBe(true);
    expect(merged.props.children).toBe('новый');
  });

  it('после чистки слотов реестр снова виден (шим размонтирован)', () => {
    const base: TopNavProps = { variant: 'search' };
    const withPatch = mergeTopNavConfig(base, new Map([['slot-a', { children: 'поле' }]]));
    expect(withPatch.props.children).toBe('поле');
    const withoutPatch = mergeTopNavConfig(base, new Map());
    expect(withoutPatch.props).toBe(base);
  });
});
