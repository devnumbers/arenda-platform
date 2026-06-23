'use client';

import { useQuery, type UseQueryResult } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { ApiError } from '@/shared/api/errors';
import { billingKeys } from './keys';
import type { components } from '@/shared/api/generated';

type TariffsResponse = components['schemas']['TariffsResponse'];
type Subscription = components['schemas']['Subscription'];

export function useTariffs(): UseQueryResult<TariffsResponse, ApiError> {
  return useQuery({
    queryKey: billingKeys.tariffs,
    queryFn: () => apiClient<TariffsResponse>('/tariffs'),
  });
}

export function useSubscription(): UseQueryResult<Subscription, ApiError> {
  return useQuery({
    queryKey: billingKeys.subscription,
    queryFn: () => apiClient<Subscription>('/subscription'),
  });
}
