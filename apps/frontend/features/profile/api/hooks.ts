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
import { mapMeResponse } from '@/entities/user';
import { mapSessionListResponse } from '@/entities/session';
import type { ActiveSession } from '@/entities/session';
import type {
  User,
  UserUpdateCommand,
  SendPhoneChangeCodeCommand,
  ChangePhoneCommand,
  VerifyCurrentEmailCommand,
  RequestNewEmailCodeCommand,
  ChangeEmailCommand,
  ResendEmailCodeCommand,
} from '@/entities/user';

/** Активные сессии вызывающего (GET /me/sessions, #728) — экран
 * «Устройства» (#730). Порядок — по свежей активности (бэк); retry
 * выключен, как у useMe: 401 угасшей сессии повторами не лечится. */
export function useSessions(): UseQueryResult<ActiveSession[], ApiError> {
  return useQuery({
    queryKey: authKeys.sessions,
    queryFn: async () => {
      const res = await apiClient<Parameters<typeof mapSessionListResponse>[0]>(
        '/me/sessions',
      );
      return mapSessionListResponse(res);
    },
    retry: false,
  });
}

/** Завершение одной чужой сессии (DELETE /me/sessions/{id}, #730);
 * текущая завершается выходом на хабе, не ревокацией. */
export function useRevokeSession(): UseMutationResult<void, ApiError, string> {
  const queryClient = useQueryClient();
  return useGuardedMutation({
    mutationFn: async (sessionId: string) => {
      await apiClient<void>(`/me/sessions/${sessionId}`, { method: 'DELETE' });
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: authKeys.sessions });
    },
  });
}

/** «Завершить все другие сессии» (POST /me/sessions/logout-others, #730):
 * текущая сессия остаётся живой. */
export function useLogoutOtherSessions(): UseMutationResult<void, ApiError, void> {
  const queryClient = useQueryClient();
  return useGuardedMutation({
    mutationFn: async () => {
      await apiClient<void>('/me/sessions/logout-others', { method: 'POST' });
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: authKeys.sessions });
    },
  });
}

export function useUpdateMe(): UseMutationResult<
  User,
  ApiError,
  UserUpdateCommand
> {
  const queryClient = useQueryClient();
  return useGuardedMutation({
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
  return useGuardedMutation({
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
  return useGuardedMutation({
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
  return useGuardedMutation({
    mutationFn: async () => {
      await apiClient<void>('/me/email/send-code', { method: 'POST' });
    },
  });
}

/** Шаг 2 флоу смены email (#721/#722, протокол #1202): код с текущего
 * адреса — сервер проверяет его, сжигает и выпускает одноразовый грант
 * (≈10 мин), адреса в запросе ещё нет. 401 неверного кода — inline на шаге. */
export function useVerifyCurrentEmail(): UseMutationResult<
  { grant: string },
  ApiError,
  VerifyCurrentEmailCommand
> {
  return useGuardedMutation({
    mutationFn: async ({ code }: VerifyCurrentEmailCommand) => {
      const res = await apiClient<{ grant: string }>('/me/email/verify-current', {
        method: 'POST',
        body: JSON.stringify({ code }),
      });
      return res;
    },
  });
}

/** Шаг 3 флоу смены почты (протокол #1202): новый адрес привязывается
 * к живому гранту из шага 2, бэк отправляет код на новый адрес (204). */
export function useRequestNewEmailCode(): UseMutationResult<
  void,
  ApiError,
  RequestNewEmailCodeCommand
> {
  return useGuardedMutation({
    mutationFn: async ({ grant, newEmail }: RequestNewEmailCodeCommand) => {
      await apiClient<void>('/me/email/request-new-email-code', {
        method: 'POST',
        body: JSON.stringify({ grant, newEmail }),
      });
    },
  });
}

/** Финальный шаг флоу смены email: код с нового адреса + грант — почта
 * меняется, ответ MeResponse обновляет кэш профиля. */
export function useChangeEmail(): UseMutationResult<
  User,
  ApiError,
  ChangeEmailCommand
> {
  const queryClient = useQueryClient();
  return useGuardedMutation({
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

/** Повторная отправка кода на новый адрес по живому гранту (#732/#733):
 * resend-плитка шага «Подтвердите новую почту». Кэш не трогает — состояние
 * /me не меняется (204); троттлинг 1 мин и бюджет 5/час — серверно. */
export function useResendEmailCode(): UseMutationResult<
  void,
  ApiError,
  ResendEmailCodeCommand
> {
  return useGuardedMutation({
    mutationFn: async ({ grant }: ResendEmailCodeCommand) => {
      await apiClient<void>('/me/email/resend-code', {
        method: 'POST',
        body: JSON.stringify({ grant }),
      });
    },
  });
}
