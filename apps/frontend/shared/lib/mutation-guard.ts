/**
 * Слот полёта мутации «один логический сабмит — один фаер» (Т2 #1120,
 * карта #1112). Окно до перерисовки: два клика одним JS-таском видят
 * один DOM — любой disabled бессилен (батчинг React, ресерч #1114);
 * закрывается только синхронный флаг вне рендера. Слот ключуется
 * переменными мутации: повтор той же команды в полёте — тот же
 * логический сабмит (join/drop решает обёртка), другая команда —
 * легитимная параллельная операция и не блокируется. TanStack Query
 * мутации намеренно не дедуплицирует — слой здесь (ресерч #1114).
 *
 * Ключ — JSON.stringify переменных: один сабмит уходит одним замыканием
 * (порядок ключей объекта стабилен), кросс-объектное сравнение не
 * цель дизайна. Чистый модуль без React — узел TDD (node-env канон).
 */

export type MutationFlights<TData = unknown> = Map<string, Promise<TData>>;

export function flightKey(variables: unknown): string {
  // typeof-проверка явная: тип JSON.stringify лжёт «string», а на
  // undefined в рантайме возвращает undefined.
  return typeof variables === 'undefined' ? 'void' : JSON.stringify(variables);
}

export function inFlight<TData>(
  flights: MutationFlights<TData>,
  variables: unknown,
): Promise<TData> | null {
  return flights.get(flightKey(variables)) ?? null;
}

/** Регистрирует полёт и снимает слот по settle (успех и ошибка — retry
 * после ошибки обязан работать). */
export function trackFlight<TData>(
  flights: MutationFlights<TData>,
  variables: unknown,
  promise: Promise<TData>,
): void {
  const key = flightKey(variables);
  flights.set(key, promise);
  const release = (): void => {
    if (flights.get(key) === promise) {
      flights.delete(key);
    }
  };
  promise.then(release, release);
}
