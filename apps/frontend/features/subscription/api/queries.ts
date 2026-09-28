import { queryOptions, type UseQueryOptions } from '@tanstack/react-query';
import { apiClient, type ApiTransport } from '@/shared/api/client';
import { nullOn404 } from '@/shared/api/null-on-404';
import type { ApiError } from '@/shared/api/errors';
import { subscriptionKeys } from '@/shared/api/query-keys';
import type { components } from '@/shared/api/dto';

type Subscription = components['schemas']['Subscription'];

/** Двойной модуль API-слоя subscription (без 'use client'): чистые
 * fetch-функции и queryOptions-фабрики канона #887 — общий источник
 * ключ+fetch для клиентских хуков и серверного префетча. Хуки — в
 * hooks.ts ('use client'). */

/** Чистый fetch подписки — общее горло хука и серверного префетча #887.
 * 404 → null — «подписки нет» (#768): хаб объектов читает подписку ради
 * лимита создания; правило — в nullOn404. */
export async function fetchSubscription(
  transport: ApiTransport = apiClient,
): Promise<Subscription | null> {
  return nullOn404(() => transport<Subscription>('/subscription'));
}

/** Опции подписки (канон #887): один источник ключ+fetch для хука
 * и серверного префетча. */
export function subscriptionQueryOptions(
  transport: ApiTransport = apiClient,
): UseQueryOptions<Subscription | null, ApiError, Subscription | null, typeof subscriptionKeys.subscription> {
  return queryOptions({
    queryKey: subscriptionKeys.subscription,
    queryFn: () => fetchSubscription(transport),
  });
}

