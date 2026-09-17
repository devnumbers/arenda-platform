'use client';

import { useCallback, useSyncExternalStore } from 'react';
import { remainingSecondsUntil } from '@/shared/lib/countdown';

/** Секунда наблюдения, квантованная вверх: стабильна внутри секунды. */
const getSecond = (): number => Math.ceil(Date.now() / 1000);

/** Тикающий остаток секунд до дедлайна (epoch ms) для подписи-таймера
 * resend-плитки: снимок — текущая секунда, квантованная вверх (снимок
 * стабилен внутри секунды, перерисовка — ровно при её смене; округление
 * вверх держит остаток в границах «00:59» сразу после пуска на 60 с).
 * Значение всегда свежее, без «застывшего» now между тиками — рестарт
 * дедлайна перерисовывает подпись сразу. Дедлайна нет — подписки и тиков
 * нет (ленивая подписка). На сервере времени нет — 0. */
export function useCountdown(deadlineMs: number | null): number {
  const subscribe = useCallback(
    (onStoreChange: () => void): (() => void) => {
      if (deadlineMs === null) {
        return () => {};
      }
      const timer = setInterval(onStoreChange, 500);
      return () => clearInterval(timer);
    },
    [deadlineMs],
  );
  const second = useSyncExternalStore(
    subscribe,
    getSecond,
    () => 0,
  );
  return remainingSecondsUntil(deadlineMs, second * 1000);
}
