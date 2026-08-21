'use client';

import {
  useMutation,
  useQueryClient,
  type UseMutationResult,
} from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import type { ApiError } from '@/shared/api/errors';
import { authKeys } from '@/shared/api/query-keys';
import { mapMeResponse } from '@/entities/user';
import type {
  User,
  UserUpdateCommand,
  SendPhoneChangeCodeCommand,
  ChangePhoneCommand,
} from '@/entities/user';

export function useUpdateMe(): UseMutationResult<
  User,
  ApiError,
  UserUpdateCommand
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (data: UserUpdateCommand) => {
      const res = await apiClient<Parameters<typeof mapMeResponse>[0]>('/me', {
        method: 'PATCH',
        body: JSON.stringify(data),
      });
      return mapMeResponse(res);
    },
    onSuccess: (data) => {
      queryClient.setQueryData(authKeys.me, data);
      queryClient.invalidateQueries({ queryKey: authKeys.all });
    },
  });
}

export function useChangePhoneSendCode(): UseMutationResult<
  void,
  ApiError,
  SendPhoneChangeCodeCommand
> {
  return useMutation({
    mutationFn: async ({ phone }: SendPhoneChangeCodeCommand) => {
      await apiClient<void>('/me/phone/send-code', {
        method: 'POST',
        body: JSON.stringify({ phone }),
      });
    },
  });
}

export function useChangePhone(): UseMutationResult<
  User,
  ApiError,
  ChangePhoneCommand
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ phone, code }: ChangePhoneCommand) => {
      const res = await apiClient<Parameters<typeof mapMeResponse>[0]>(
        '/me/phone/change',
        {
          method: 'POST',
          body: JSON.stringify({ phone, code }),
        },
      );
      return mapMeResponse(res);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: authKeys.all });
    },
  });
}
