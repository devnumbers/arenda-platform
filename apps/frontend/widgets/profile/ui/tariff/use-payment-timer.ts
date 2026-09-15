'use client';

import { useEffect, useRef, useState } from 'react';

/** Тикающая «сейчас» для отсчётов живой pending-оплаты (#616): раз в секунду
 * обновляет время, пока `enabled`; по истечении expiresAt интервал
 * останавливается и зовётся onExpired (refetch: бэк пометит платёж failed,
 * правило #620 — на «00:00» останавливаемся, а не прячем отсчёт). */
export function usePaymentTimer(
  expiresAt: string,
  enabled: boolean,
  onExpired?: () => void,
): Date {
  const [now, setNow] = useState(() => new Date());
  const onExpiredRef = useRef(onExpired);

  useEffect(() => {
    onExpiredRef.current = onExpired;
  }, [onExpired]);

  useEffect(() => {
    if (!enabled) {
      return;
    }
    const expiresMs = new Date(expiresAt).getTime();
    const timer = setInterval(() => {
      const current = new Date();
      setNow(current);
      if (current.getTime() >= expiresMs) {
        clearInterval(timer);
        onExpiredRef.current?.();
      }
    }, 1000);
    return () => clearInterval(timer);
  }, [enabled, expiresAt]);

  return now;
}
