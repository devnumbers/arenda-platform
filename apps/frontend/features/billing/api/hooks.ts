'use client';

import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { billingKeys } from './keys';
import type { components } from '@/shared/api/generated';

type TariffsResponse = components['schemas']['TariffsResponse'];
type Subscription = components['schemas']['Subscription'];

export function useTariffs() {
  return useQuery({
    queryKey: billingKeys.tariffs,
    queryFn: () => apiClient<TariffsResponse>('/tariffs'),
  });
}

export function useSubscription() {
  return useQuery({
    queryKey: billingKeys.subscription,
    queryFn: () => apiClient<Subscription>('/subscription'),
  });
}
