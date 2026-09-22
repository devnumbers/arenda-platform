'use client';

import { useQuery, type UseQueryResult } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { nullOn404 } from '@/shared/api/null-on-404';
import type { ApiError } from '@/shared/api/errors';
import { subscriptionKeys } from '@/shared/api/query-keys';
import type { components } from '@/shared/api/dto';

type Subscription = components['schemas']['Subscription'];

/** 404 → null — «подписки нет» (#768): хаб объектов читает подписку ради
 * лимита создания; правило — в nullOn404. */
export function useSubscription(): UseQueryResult<Subscription | null, ApiError> {
  return useQuery({
    queryKey: subscriptionKeys.subscription,
    queryFn: () => nullOn404(() => apiClient<Subscription>('/subscription')),
  });
}
