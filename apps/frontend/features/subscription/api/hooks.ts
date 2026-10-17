'use client';

import { useQuery, type UseQueryResult } from '@tanstack/react-query';
import type { ApiError } from '@/shared/api/errors';
import type { components } from '@/shared/api/dto';
import { isPaidTariff } from '@/entities/user';
import { subscriptionQueryOptions } from './queries';

type Subscription = components['schemas']['Subscription'];

export function useSubscription(): UseQueryResult<Subscription | null, ApiError> {
  return useQuery(subscriptionQueryOptions());
}

/** Платный ли тариф у читателя (канон optimistic-умолчания экранов,
 * карта #1459): подписка ещё не загрузилась или её нет — платный, basic —
 * нет. Истиной остаётся бекенд-гейт (402 tariff_required) — фронтовое
 * умолчание лишь не гасит UI на сбое чтения; для решений с ценой ошибки
 * (гейт списка участников, D2 #1313) берите useSubscription и проверяйте
 * resolved-флаги сами. */
export function usePaidTariff(): boolean {
  const subscriptionQuery = useSubscription();
  return subscriptionQuery.data
    ? isPaidTariff(subscriptionQuery.data.tariff.name)
    : true;
}
