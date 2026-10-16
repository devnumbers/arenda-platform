'use client';

import { useQuery, type UseQueryResult } from '@tanstack/react-query';
import type { ApiError } from '@/shared/api/errors';
import type { components } from '@/shared/api/dto';
import { subscriptionQueryOptions } from './queries';

type Subscription = components['schemas']['Subscription'];

export function useSubscription(): UseQueryResult<Subscription | null, ApiError> {
  return useQuery(subscriptionQueryOptions());
}
