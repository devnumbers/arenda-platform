'use client';

import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { ApiError } from '@/shared/api/errors';
import { authKeys } from './keys';
import type { components } from '@/shared/api/generated';
import type { User } from '@/entities/user/model/types';
import { mapMeResponse } from '@/entities/user/model/mappers';

type MeResponse = components['schemas']['MeResponse'];
type SendCodeRequest = components['schemas']['SendCodeRequest'];
type SendCodeResponse = components['schemas']['SendCodeResponse'];
type VerifyCodeRequest = components['schemas']['VerifyCodeRequest'];

export function useMe(): UseQueryResult<User, ApiError> {
  return useQuery({
    queryKey: authKeys.me,
    queryFn: async () => {
      const res = await apiClient<MeResponse>('/me');
      return mapMeResponse(res);
    },
    retry: false,
  });
}

export function useSendCode(): UseMutationResult<
  SendCodeResponse,
  ApiError,
  SendCodeRequest
> {
  return useMutation({
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
  return useMutation({
    mutationFn: (data: VerifyCodeRequest) =>
      apiClient<MeResponse>('/auth/verify', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: authKeys.me });
    },
  });
}

export function useLogout(): UseMutationResult<void, ApiError, void> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => apiClient<void>('/auth/logout', { method: 'POST' }),
    onSuccess: () => {
      queryClient.removeQueries({ queryKey: authKeys.me });
    },
  });
}
