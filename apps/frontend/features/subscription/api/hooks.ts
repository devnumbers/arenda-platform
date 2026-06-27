'use client';

import { useQuery, type UseQueryResult } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { ApiError } from '@/shared/api/errors';
import { subscriptionKeys } from './keys';
import type { components } from '@/shared/api/generated';

type Subscription = components['schemas']['Subscription'];

export function useSubscription(): UseQueryResult<Subscription, ApiError> {
  return useQuery({
    queryKey: subscriptionKeys.subscription,
    queryFn: () => apiClient<Subscription>('/subscription'),
  });
}
