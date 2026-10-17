import { describe, expect, it } from 'vitest';
import { planWheelSync } from './wheel-sync';

/** Решение синка колеса (план 049): позиция ленты после приписки рядов
 * сверху (перекройка месяцев, доращивка годов назад) восстанавливается
 * прямой установкой — никогда анимированным роллом и никогда ставкой на
 * движковый реснап. Ролл остаётся только обычной докруткой значения
 * (клавиатура, внешний выбор), где колесо действительно едет к ряду. */
describe('planWheelSync — что делает синк при следующем рендере', () => {
  const base = {
    instant: false,
    syncFrom: 9 as number | null,
    valueIndex: 9,
    valueChanged: false,
    grewAtStart: false,
  };

  it('индекс не сдвинулся и лента не росла — скип: позиция уже правильная', () => {
    expect(planWheelSync(base)).toBe('skip');
  });

  it('первое монтирование — прямая установка: выбранный ряд уже в центре', () => {
    expect(planWheelSync({ ...base, instant: true, syncFrom: null, valueIndex: 9 })).toBe(
      'direct',
    );
  });

  it('перекройка ленты без смены значения (разблокировка месяцев года) — прямая установка', () => {
    // [октябрь..декабрь] → полный год: октябрь переехал из ряда 0 в ряд 9,
    // значение то же. Ставка на реснап — source #1429 (WebKit промахивается
    // или не реснапится вовсе).
    expect(planWheelSync({ ...base, syncFrom: 0, valueIndex: 9 })).toBe('direct');
  });

  it('доращивка назад совпала со сменой значения (settle-коммит года) — прямая установка, не ролл', () => {
    // Settle коммитил год (1928) и доращивку (+400 рядов сверху) одним
    // рендером: колесо УЖЕ стоит на своём году, сдвинулось только
    // смещение. Ролл здесь катил ~19 000px через века (репро 17.10).
    expect(
      planWheelSync({
        ...base,
        syncFrom: 134,
        valueIndex: 436,
        valueChanged: true,
        grewAtStart: true,
      }),
    ).toBe('direct');
  });

  it('доращивка назад без смены значения — прямая установка (перекройка смещения)', () => {
    expect(
      planWheelSync({ ...base, syncFrom: 36, valueIndex: 436, grewAtStart: true }),
    ).toBe('direct');
  });

  it('обычная смена значения (клавиатура, внешний выбор) — плавный ролл', () => {
    expect(
      planWheelSync({ ...base, syncFrom: 5, valueIndex: 7, valueChanged: true }),
    ).toBe('roll');
  });

  it('после доращивки ролл не возвращается, пока не сменится значение без доращивки', () => {
    // Две доращивки подряд: обе прямые.
    expect(
      planWheelSync({
        ...base,
        syncFrom: 436,
        valueIndex: 836,
        valueChanged: true,
        grewAtStart: true,
      }),
    ).toBe('direct');
  });
});
