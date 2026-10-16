'use client';

import {
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from '@tanstack/react-query';
import { useGuardedMutation } from '@/shared/lib/hooks/use-guarded-mutation';
import { apiClient } from '@/shared/api/client';
import type { ApiError } from '@/shared/api/errors';
import { authKeys } from '@/shared/api/query-keys';
import type { components } from '@/shared/api/dto';
import type { User } from '@/entities/user';
import { meQueryOptions } from './queries';

type MeResponse = components['schemas']['MeResponse'];
type SendCodeRequest = components['schemas']['SendCodeRequest'];
type SendCodeResponse = components['schemas']['SendCodeResponse'];
type VerifyCodeRequest = components['schemas']['VerifyCodeRequest'];

export function useMe(): UseQueryResult<User, ApiError> {
  // Без локального retry: 4xx (включая 401) гасит глобальный предикат
  // (#769), сетевые/5xx сбои ретраются по общему дефолту.
  return useQuery(meQueryOptions());
}

export function useSendCode(): UseMutationResult<
  SendCodeResponse,
  ApiError,
  SendCodeRequest
> {
  return useGuardedMutation({
    mutationFn: (data: SendCodeRequest) =>
      apiClient<SendCodeResponse>('/auth/send', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
  });
}

export function useVerifyCode(): UseMutationResult<
  MeResponse,
  ApiError,
  VerifyCodeRequest
> {
  const queryClient = useQueryClient();
  return useGuardedMutation({
    mutationFn: (data: VerifyCodeRequest) =>
      apiClient<MeResponse>('/auth/verify', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: authKeys.me });
    },
  });
}

export function useLogout(): UseMutationResult<void, ApiError, void> {
  const queryClient = useQueryClient();
  return useGuardedMutation({
    mutationFn: () => apiClient<void>('/auth/logout', { method: 'POST' }),
    onSuccess: () => {
      // Drop the whole cache so the next user in this browser session never
      // sees the previous user's queries (popups, properties, etc.).
      queryClient.clear();
    },
  });
}
