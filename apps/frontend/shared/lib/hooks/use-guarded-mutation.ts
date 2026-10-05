'use client';

import { useRef } from 'react';
import {
  useMutation,
  type UseMutationOptions,
  type UseMutationResult,
} from '@tanstack/react-query';
import { inFlight, trackFlight } from '@/shared/lib/mutation-guard';

/**
 * useMutation с машинным гардом двойного сабмита (Т2 #1120, карта
 * #1112): окно до перерисовки — два клика одним JS-таском — закрыто
 * синхронным слотом полёта ([[mutation-guard]]), до TanStack и до
 * рендера. Канонизация startSendOnce из #1099: одна точка на все
 * поверхности вместо ручных рефов. Семантика по спеллингу: повтор
 * `.mutate` той же команды в полёте отбрасывается молча (first wins),
 * `.mutateAsync` возвращает промис полёта (join — визард ждёт результат
 * того же сабмита, а не падает на undefined); другая команда летит
 * параллельно. Наружу обычный UseMutationResult — вызывные места и
 * экраны не меняются. Правило размещения: хук в shared/lib/hooks,
 * генерики выводятся из mutationFn, как у useMutation.
 */

type GuardedMutationOptions<TData, TError, TVariables, TContext> =
  UseMutationOptions<TData, TError, TVariables, TContext> & {
    /** Выключить гард для хука с легитимными одинаковыми параллельными
     * сабмитами (сегодня таких нет; заложено на будущее). */
    readonly noSubmitGuard?: boolean;
  };

export function useGuardedMutation<TData = unknown, TVariables = void, TError = Error, TContext = unknown>(
  options: GuardedMutationOptions<TData, TError, TVariables, TContext>,
): UseMutationResult<TData, TError, TVariables, TContext> {
  const { noSubmitGuard = false, ...mutationOptions } = options;
  const mutation = useMutation(mutationOptions);
  const flights = useRef(new Map<string, Promise<TData>>());

  if (noSubmitGuard) {
    return mutation;
  }

  const mutate: UseMutationResult<TData, TError, TVariables, TContext>['mutate'] = (
    variables,
    mutateOptions,
  ) => {
    // drop, first wins: тот же сабмит в полёте молча не фаерится —
    // per-call колбэки сброса не срабатывают (канон TanStack: они
    // вообще фаерятся один раз на вызов).
    if (inFlight(flights.current, variables) !== null) {
      return;
    }
    const promise = mutation.mutateAsync(variables, mutateOptions);
    trackFlight(flights.current, variables, promise);
    // Семантика mutate — проглотить ошибку (иначе unhandled rejection:
    // mutateAsync внутри); .catch возвращает промис — глушим и его.
    void promise.catch(() => {});
  };

  const mutateAsync: UseMutationResult<TData, TError, TVariables, TContext>['mutateAsync'] = (
    variables,
    mutateOptions,
  ) => {
    const flying = inFlight(flights.current, variables);
    if (flying !== null) {
      return flying;
    }
    const promise = mutation.mutateAsync(variables, mutateOptions);
    trackFlight(flights.current, variables, promise);
    return promise;
  };

  return { ...mutation, mutate, mutateAsync };
}
