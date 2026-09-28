'use client';

import { useMemo, type ReactNode, type JSX } from 'react';
import { hydrate, useQueryClient, type HydrateOptions, type DehydratedState } from '@tanstack/react-query';

/**
 * Граница гидратации серверного префетча #887 — HydrationBoundary, который
 * гидратирует ВЕСЬ набор синхронно в рендере, включая запросы, уже
 * существующие в клиентском кэше.
 *
 * Зачем: библиотечный HydrationBoundary гидратирует новые запросы в рендере,
 * а существующие — в useEffect. Две проблемы этого сплита для канона #887:
 * на сервере эффекты не запускаются вовсе — запрос, созданный пустым
 * наблюдателем оболочки ДО границы (пилюля профиля читает useMe над
 * страницей), остаётся пустым в SSR-HTML, и потребители ниже границы рисуют
 * кадр без данных (находка walkthrough #887: лента истории без «(Вы)»,
 * React #418); на клиенте наблюдатель оболочки подписывается синхронно в
 * рендере (useSyncExternalStore), так что сплит «по счётчику наблюдателей»
 * даёт разный результат на сервере и клиенте — сам по себе источник
 * гидратационного расхождения.
 *
 * Безопасность синхронной гидратации существующих: уведомления наблюдателей
 * у react-query батчатся (notifyManager) и уходят за пределы рендера;
 * гонка с живыми данными закрыта внутри hydrate() — существующий запрос с
 * более свежим dataUpdatedAt (SSE-кадр) не перезатирается (гарды
 * dataUpdatedAt/dehydratedAt, фиксируются тестом server-prefetch-core).
 */
export function SyncHydrationBoundary({
  state,
  options,
  children,
}: Readonly<{
  state: DehydratedState | undefined;
  options?: HydrateOptions;
  children: ReactNode;
}>): JSX.Element {
  const queryClient = useQueryClient();

  useMemo(() => {
    if (state === undefined || typeof state !== 'object') {
      return;
    }
    hydrate(queryClient, state, options);
  }, [queryClient, state, options]);

  return <>{children}</>;
}
