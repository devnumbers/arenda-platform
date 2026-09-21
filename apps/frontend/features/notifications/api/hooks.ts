'use client';

import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
  type UseInfiniteQueryResult,
  type UseMutationResult,
  type UseQueryResult,
} from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import type { ApiError } from '@/shared/api/errors';
import {
  mapNotification,
  mapNotificationDetail,
  mapCategoryPreferences,
  type Notification,
  type NotificationDetail,
  type NotificationCategoryPreferences,
} from '@/entities/notification';
import { notificationKeys } from '@/shared/api/query-keys';
import { keysetNextPageParam } from '@/shared/lib/keyset';
import type { components } from '@/shared/api/dto';

type NotificationsPageDto = components['schemas']['NotificationsPageResponse'];
type UnreadCountDto = components['schemas']['UnreadCountResponse'];
type NotificationDetailDto = components['schemas']['NotificationDetailResponse'];
type PreferencesDto = components['schemas']['NotificationPreferencesResponse'];
type PreferencesRequestDto =
  components['schemas']['NotificationPreferencesRequest'];

/** Порция ленты: контракт GET /notifications (#743) — порции по 50,
 * потолок сервера 100. */
export const NOTIFICATIONS_PAGE_SIZE = 50;

/**
 * Порция ленты уведомлений: строки плюс keyset-продолжение — opaque-курсор
 * следующей порции, null = порций больше нет (удалённые строки не приходят,
 * #743).
 */
export type NotificationsPageData = {
  readonly items: Notification[];
  readonly nextCursor: string | null;
};

/**
 * Лента уведомлений (#744): порции по 50 keyset-курсором (канон #597),
 * newest-first. unreadOnly — фильтр «Непрочитанные», часть ключа: тап по
 * чипу читает ленту с другим ключом, кэши обоих срезов живут независимо.
 */
export function useNotificationsFeed(
  unreadOnly: boolean,
): UseInfiniteQueryResult<Notification[], ApiError> {
  return useInfiniteQuery({
    queryKey: notificationKeys.list(unreadOnly),
    queryFn: ({ pageParam }) => fetchNotificationsPage({ unreadOnly, cursor: pageParam }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: keysetNextPageParam,
    select: (data) => data.pages.flatMap((page) => page.items),
  });
}

/** Общее горло порции GET /notifications (#743): cursor — keyset-продолжение
 * прошлого ответа, undefined читает с начала. */
async function fetchNotificationsPage(params: {
  unreadOnly: boolean;
  cursor?: string;
}): Promise<NotificationsPageData> {
  const query = new URLSearchParams();
  if (params.unreadOnly) {
    query.set('unread', 'true');
  }
  query.set('limit', String(NOTIFICATIONS_PAGE_SIZE));
  if (params.cursor) {
    query.set('cursor', params.cursor);
  }
  const response = await apiClient<NotificationsPageDto>(
    `/notifications?${query.toString()}`,
  );
  return {
    items: response.items.map(mapNotification),
    nextCursor: response.next_cursor ?? null,
  };
}

/** Счётчик непрочитанных (бейдж чипа «Непрочитанные N» и бейджей
 * навигации) — GET /notifications/unread-count. refetchOnWindowFocus —
 * локальный: бейдж живёт весь сеанс (не только экран ленты), возврат во
 * вкладку перечитывает счёт (глобальный дефолт канона выключен, #747). */
export function useUnreadNotificationsCount(): UseQueryResult<number, ApiError> {
  return useQuery({
    queryKey: notificationKeys.unreadCount(),
    queryFn: async () => {
      const response = await apiClient<UnreadCountDto>('/notifications/unread-count');
      return response.count;
    },
    refetchOnWindowFocus: true,
  });
}

/**
 * Страница уведомления (GET /notifications/{id}, #745): строка плюс живые
 * действия читателя (вычислены при чтении, решение #737). staleTime 0 —
 * кэш никогда не свежий: возврат с выполненного действия перемонтирует
 * экран, ремаунт перечитывает — выполненные кнопки пропадают (макет
 * 2333:184048, аннотация «после действия кнопки пропадают»).
 */
export function useNotificationDetail(id: string): UseQueryResult<NotificationDetail, ApiError> {
  return useQuery({
    queryKey: notificationKeys.detail(id),
    queryFn: async () => {
      const response = await apiClient<NotificationDetailDto>(`/notifications/${id}`);
      return mapNotificationDetail(response);
    },
    staleTime: 0,
  });
}

/** Прочтение одного уведомления (POST /notifications/{id}/read,
 * идемпотентно, #743): глушит точку строки и снижает счётчик. */
export function useMarkNotificationRead(): UseMutationResult<void, ApiError, string> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      await apiClient<void>(`/notifications/${id}/read`, { method: 'POST' });
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: notificationKeys.all });
    },
  });
}

/** «Прочитать все» (POST /notifications/read-all, #743): подтверждения
 * не требует — обратимая отметка, на макете за ней следует успех-попап
 * (2329-148575). */
export function useMarkAllNotificationsRead(): UseMutationResult<void, ApiError, void> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      await apiClient<void>('/notifications/read-all', { method: 'POST' });
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: notificationKeys.all });
    },
  });
}

/** Удаление одного уведомления (DELETE /notifications/{id}, мягкое, #743). */
export function useDeleteNotification(): UseMutationResult<void, ApiError, string> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      await apiClient<void>(`/notifications/${id}`, { method: 'DELETE' });
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: notificationKeys.all });
    },
  });
}

/** «Удалить все уведомления» (DELETE /notifications, #743): мягко сносит
 * ленту читателя; подтверждение — на экране (шит 2329-152107). */
export function useDeleteAllNotifications(): UseMutationResult<void, ApiError, void> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      await apiClient<void>('/notifications', { method: 'DELETE' });
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: notificationKeys.all });
    },
  });
}

/**
 * Матрица email-настроек аккаунта (GET /notification-preferences, #743):
 * одна конфигурация на все устройства, нет строки — всё включено (решение
 * #738, ADR 0058). Тариф и Системные бэк не отдаёт — они всегда включены.
 */
export function useEmailNotificationPreferences(): UseQueryResult<
  NotificationCategoryPreferences,
  ApiError
> {
  return useQuery({
    queryKey: notificationKeys.emailPreferences(),
    queryFn: async () => {
      const response = await apiClient<PreferencesDto>('/notification-preferences');
      return mapCategoryPreferences(response.email);
    },
  });
}

/**
 * Замена email-матрицы (PUT /notification-preferences, #743) —
 * оптимистично: тумблер двигается сразу, при ошибке снимок
 * восстанавливается (требование #746). Серверный ответ — истина:
 * onSuccess кладёт его в кэш, onSettled перечитывает.
 */
export function useUpdateEmailPreferences(): UseMutationResult<
  NotificationCategoryPreferences,
  ApiError,
  NotificationCategoryPreferences
> {
  const queryClient = useQueryClient();
  const queryKey = notificationKeys.emailPreferences();
  return useMutation({
    mutationFn: async (email: NotificationCategoryPreferences) => {
      const body: PreferencesRequestDto = { email };
      const response = await apiClient<PreferencesDto>('/notification-preferences', {
        method: 'PUT',
        body: JSON.stringify(body),
      });
      return mapCategoryPreferences(response.email);
    },
    onMutate: async (next) => {
      await queryClient.cancelQueries({ queryKey });
      const previous = queryClient.getQueryData<NotificationCategoryPreferences>(queryKey);
      queryClient.setQueryData(queryKey, next);
      return { previous };
    },
    onError: (_error, _next, context) => {
      if (context?.previous !== undefined) {
        queryClient.setQueryData(queryKey, context.previous);
      }
    },
    onSuccess: (saved) => {
      queryClient.setQueryData(queryKey, saved);
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey });
    },
  });
}
