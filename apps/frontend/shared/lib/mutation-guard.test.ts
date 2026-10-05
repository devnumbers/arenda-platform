import { describe, expect, it } from 'vitest';
import { flightKey, inFlight, trackFlight } from './mutation-guard';

/** Тикет Т2 #1120: чистая координация «один логический сабмит — один
 * фаер» — синхронный слот полёта по ключу переменных (окно до
 * перерисовки: два клика одним JS-таском видят один DOM, disabled
 * бессилен — ресерчи #1113/#1114). Node-env канон: без рендера/DOM. */

const deferred = <T>(): { promise: Promise<T>; resolve: (v: T) => void; reject: (e: unknown) => void } => {
  let resolve!: (v: T) => void;
  let reject!: (e: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
};

const tick = (): Promise<void> => new Promise((resolve) => setTimeout(resolve, 0));

describe('flightKey', () => {
  it('одинаковые команды — один ключ (одна логическая операция)', () => {
    const command = { name: 'Страхование', amountKopecks: 2500 };
    expect(flightKey(command)).toBe(flightKey({ name: 'Страхование', amountKopecks: 2500 }));
  });

  it('разные команды — разные ключи (легитимные параллельные операции)', () => {
    expect(flightKey({ paymentId: 'a' })).not.toBe(flightKey({ paymentId: 'b' }));
  });

  it('мутация без переменных — ключ «void»', () => {
    expect(flightKey(undefined)).toBe('void');
  });
});

describe('inFlight / trackFlight', () => {
  it('до старта полёта нет, в полёте есть, после settle слот снят и ре-захват работает', async () => {
    const flights = new Map<string, Promise<unknown>>();
    const { promise, resolve } = deferred<string>();

    expect(inFlight(flights, { id: 1 })).toBeNull();
    trackFlight(flights, { id: 1 }, promise);
    expect(inFlight(flights, { id: 1 })).toBe(promise);

    resolve('готово');
    await tick();
    expect(inFlight(flights, { id: 1 })).toBeNull();

    const second = deferred<string>();
    trackFlight(flights, { id: 1 }, second.promise);
    expect(inFlight(flights, { id: 1 })).toBe(second.promise);
    second.resolve('ещё раз');
    await tick();
    expect(inFlight(flights, { id: 1 })).toBeNull();
  });

  it('ошибка мутации тоже снимает слот (retry после ошибки жив)', async () => {
    const flights = new Map<string, Promise<unknown>>();
    const { promise, reject } = deferred<string>();

    trackFlight(flights, 'cmd', promise);
    reject(new Error('упало'));
    await tick();
    expect(inFlight(flights, 'cmd')).toBeNull();
  });

  it('разные переменные летят параллельно — слоты независимы', () => {
    const flights = new Map<string, Promise<unknown>>();
    const a = deferred<string>();
    const b = deferred<string>();

    trackFlight(flights, { paymentId: 'a' }, a.promise);
    trackFlight(flights, { paymentId: 'b' }, b.promise);
    expect(inFlight(flights, { paymentId: 'a' })).toBe(a.promise);
    expect(inFlight(flights, { paymentId: 'b' })).toBe(b.promise);
  });
});
