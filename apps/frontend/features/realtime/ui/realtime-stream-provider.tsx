'use client';

import { useEffect, type JSX } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import type { EventStreamVisibility } from '@/shared/api/sse-client';
import { realtimeHandlers } from '../api/realtime-handlers';
import { connectRealtimeStream } from '../api/realtime-stream';

/** Браузерная видимость вкладки для гварда соединения. */
function documentVisibility(): EventStreamVisibility {
  return {
    isVisible: () => document.visibilityState === 'visible',
    onChange: (listener) => {
      document.addEventListener('visibilitychange', listener);
      return () => document.removeEventListener('visibilitychange', listener);
    },
  };
}

/**
 * Живой слой realtime-инвалидации (карта #714, тикет #717): одно
 * SSE-соединение на вкладку (EventSource на /api/realtime/stream,
 * cookie-сессия), кадры entity.changed — префиксные инвалидации react-query
 * по словарю сущностей ADR 0062. Соединение — под гвардом видимости:
 * скрытая вкладка источник закрывает (бережёт бюджет хаба — 8 соединений
 * на пользователя на два стрима), показ открывает заново, а открытие
 * перечитывает живое — пропущенное в скрытом периоде догоняется само.
 * Монтируется раз на сессию вкладки в ScreenLayout; рвётся при unmount.
 */
export function RealtimeStreamProvider(): JSX.Element | null {
  const queryClient = useQueryClient();

  useEffect(() => {
    return connectRealtimeStream({
      createSource: () => new EventSource('/api/realtime/stream'),
      handlers: realtimeHandlers(queryClient),
      visibility: documentVisibility(),
    });
  }, [queryClient]);

  return null;
}
