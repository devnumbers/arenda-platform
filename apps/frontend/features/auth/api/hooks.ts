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
type SendPhoneCodeRequest = components['schemas']['SendPhoneCodeRequest'];
type VerifyPhoneCodeRequest = components['schemas']['VerifyPhoneCodeRequest'];

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

export function useSendPhoneCode(): UseMutationResult<
  void,
  ApiError,
  SendPhoneCodeRequest
> {
  return useMutation({
    mutationFn: (data: SendPhoneCodeRequest) =>
      apiClient<void>('/auth/phone/send', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
  });
}

export function useVerifyPhoneCode(): UseMutationResult<
  MeResponse,
  ApiError,
  VerifyPhoneCodeRequest
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: VerifyPhoneCodeRequest) =>
      apiClient<MeResponse>('/auth/phone/verify', {
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
