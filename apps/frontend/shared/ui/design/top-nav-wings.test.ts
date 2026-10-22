import { describe, expect, it } from 'vitest';
import { topNavWingClasses, topNavWingUser } from './top-nav-wings';

/** Канон тестов фронта — чистая логика без DOM (vitest.config.mts):
 * решается только «какие классы видимости получают крылья» и «что пишет
 * крыло профиля»; сам рендер — экранная приёмка и e2e (прецедент
 * circle-photo.test.ts). */

describe('topNavWingClasses — видимость крыльев по ярусам', () => {
  const base = {
    hasLeading: false,
    overlay: false,
    hideWingsBelowDesktop: false,
    mobileWings: false,
    persistentWings: false,
  } as const;

  it('хаб (mobileWings, без leading) — крылья на мобайле и планшете+ПК', () => {
    const c = topNavWingClasses({ ...base, mobileWings: true });
    expect(c.wingsMobileClass).toBe('flex');
    expect(c.wingsTierClass).toBe('tablet:flex');
    expect(c.desktopWingsClass).toBe('');
  });

  it('подэкран с leading — крылья только на ПК', () => {
    const c = topNavWingClasses({ ...base, hasLeading: true });
    expect(c.wingsMobileClass).toBe('hidden');
    expect(c.wingsTierClass).toBe('desktop:flex');
  });

  it('hideWingsBelowDesktop («Профиль») — крылья только на ПК', () => {
    const c = topNavWingClasses({ ...base, hideWingsBelowDesktop: true });
    expect(c.wingsMobileClass).toBe('hidden');
    expect(c.wingsTierClass).toBe('desktop:flex');
  });

  it('overlay — крыльев нет ниже ПК, на ПК рисует сама поверхность', () => {
    const c = topNavWingClasses({ ...base, overlay: true, mobileWings: true });
    expect(c.wingsMobileClass).toBe('hidden');
    expect(c.wingsTierClass).toBe('desktop:flex');
  });

  it('постоянное крыло ПК: у хаба крыло страницы гасится на ПК, планшет живёт по-прежнему', () => {
    const c = topNavWingClasses({ ...base, mobileWings: true, persistentWings: true });
    expect(c.wingsMobileClass).toBe('flex');
    expect(c.wingsTierClass).toBe('tablet:flex');
    expect(c.desktopWingsClass).toBe('desktop:hidden');
  });

  it('постоянное крыло ПК: у подэкрана с leading ПК-крыло тоже гасится', () => {
    const c = topNavWingClasses({ ...base, hasLeading: true, persistentWings: true });
    expect(c.wingsTierClass).toBe('desktop:flex');
    expect(c.desktopWingsClass).toBe('desktop:hidden');
  });

  it('overlay не гасится постоянным крылом — поверхность рисует своё', () => {
    const c = topNavWingClasses({ ...base, overlay: true, persistentWings: true });
    expect(c.desktopWingsClass).toBe('');
  });

  it('без провайдера (ui-kit, логин) — прежнее поведение без гашения', () => {
    const c = topNavWingClasses({ ...base, hasLeading: true });
    expect(c.desktopWingsClass).toBe('');
  });
});

describe('topNavWingUser — имя и pending крыла профиля', () => {
  it('вне провайдера — плейсхолдер без pending', () => {
    expect(topNavWingUser(null)).toEqual({ wingName: 'Пользователь', pending: false });
  });

  it('pending (useMe в полёте) — имени нет, скелетон', () => {
    expect(topNavWingUser({ pending: true })).toEqual({ wingName: undefined, pending: true });
  });

  it('имя есть — оно и пишется', () => {
    expect(topNavWingUser({ name: 'Иван' })).toEqual({ wingName: 'Иван', pending: false });
  });

  it('пустая строка имени — плейсхолдер (контракт допускает пустую строку)', () => {
    expect(topNavWingUser({ name: '' }).wingName).toBe('Пользователь');
    expect(topNavWingUser({}).wingName).toBe('Пользователь');
  });
});
