'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { authKeys } from './keys';
import type { components } from '@/shared/api/generated';

type MeResponse = components['schemas']['MeResponse'];
type SendPhoneCodeRequest = components['schemas']['SendPhoneCodeRequest'];
type VerifyPhoneCodeRequest = components['schemas']['VerifyPhoneCodeRequest'];

export function useMe() {
  return useQuery({
    queryKey: authKeys.me,
    queryFn: () => apiClient<MeResponse>('/me'),
    retry: false,
  });
}

export function useSendPhoneCode() {
  return useMutation({
    mutationFn: (data: SendPhoneCodeRequest) =>
      apiClient<void>('/auth/phone/send', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
  });
}

export function useVerifyPhoneCode() {
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

export function useLogout() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => apiClient<void>('/auth/logout', { method: 'POST' }),
    onSuccess: () => {
      queryClient.removeQueries({ queryKey: authKeys.me });
    },
  });
}
