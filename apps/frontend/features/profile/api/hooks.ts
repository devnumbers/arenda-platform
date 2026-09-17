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
  ConfirmCurrentEmailCommand,
  ChangeEmailCommand,
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
      void queryClient.invalidateQueries({ queryKey: authKeys.all });
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
      void queryClient.invalidateQueries({ queryKey: authKeys.all });
    },
  });
}

/** Шаг 1 флоу смены почты (#722): код на текущий адрес. Тела нет — бэк
 * берёт адрес из сессии (204). */
export function useEmailChangeSendCode(): UseMutationResult<
  void,
  ApiError,
  void
> {
  return useMutation({
    mutationFn: async () => {
      await apiClient<void>('/me/email/send-code', { method: 'POST' });
    },
  });
}

/** Шаг 2: код с текущего адреса + новый адрес одним запросом (#721) —
 * в ответ одноразовый грант (≈10 мин), бэк отправляет код на новый адрес. */
export function useConfirmCurrentEmail(): UseMutationResult<
  { grant: string },
  ApiError,
  ConfirmCurrentEmailCommand
> {
  return useMutation({
    mutationFn: async ({ newEmail, code }: ConfirmCurrentEmailCommand) => {
      const res = await apiClient<{ grant: string }>('/me/email/confirm-current', {
        method: 'POST',
        body: JSON.stringify({ newEmail, code }),
      });
      return res;
    },
  });
}

/** Шаг 3: код с нового адреса + грант — почта меняется, ответ MeResponse
 * обновляет кэш профиля. */
export function useChangeEmail(): UseMutationResult<
  User,
  ApiError,
  ChangeEmailCommand
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ grant, code }: ChangeEmailCommand) => {
      const res = await apiClient<Parameters<typeof mapMeResponse>[0]>(
        '/me/email/change',
        {
          method: 'POST',
          body: JSON.stringify({ grant, code }),
        },
      );
      return mapMeResponse(res);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: authKeys.all });
    },
  });
}
