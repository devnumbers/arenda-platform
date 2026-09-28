import { queryOptions, type UseQueryOptions } from '@tanstack/react-query';
import { apiClient, type ApiTransport } from '@/shared/api/client';
import type { ApiError } from '@/shared/api/errors';
import type { components } from '@/shared/api/dto';
import {
  mapNotification,
  mapNotificationDetail,
  type Notification,
  type NotificationDetail,
} from '@/entities/notification';
import { notificationKeys } from '@/shared/api/query-keys';
import { keysetNextPageParam } from '@/shared/lib/keyset';

type NotificationsPageDto = components['schemas']['NotificationsPageResponse'];
type UnreadCountDto = components['schemas']['UnreadCountResponse'];
type NotificationDetailDto = components['schemas']['NotificationDetailResponse'];

/** Порция ленты уведомлений: строки плюс keyset-продолжение — opaque-курсор
 * следующей порции, null = порций больше нет (удалённые строки не приходят,
 * #743). */
export type NotificationsPageData = {
  readonly items: Notification[];
  readonly nextCursor: string | null;
};

/** Размер порции ленты (#743). */
const NOTIFICATIONS_PAGE_SIZE = 50;

/** Двойной модуль API-слоя notifications (без 'use client'): чистые
 * fetch-функции и queryOptions-фабрики канона #887 — общий источник
 * ключ+fetch для клиентских хуков и серверного префетча. Хуки — в
 * hooks.ts ('use client'). */

/** Общее горло порции GET /notifications (#743): cursor — keyset-продолжение
 * прошлого ответа, undefined читает с начала. */
async function fetchNotificationsPage(params: {
  unreadOnly: boolean;
  cursor?: string;
  transport?: ApiTransport;
}): Promise<NotificationsPageData> {
  const transport = params.transport ?? apiClient;
  const query = new URLSearchParams();
  if (params.unreadOnly) {
    query.set('unread', 'true');
  }
  query.set('limit', String(NOTIFICATIONS_PAGE_SIZE));
  if (params.cursor) {
    query.set('cursor', params.cursor);
  }
  const response = await transport<NotificationsPageDto>(
    `/notifications?${query.toString()}`,
  );
  return {
    items: response.items.map(mapNotification),
    nextCursor: response.next_cursor ?? null,
  };
}


export type NotificationsFeedQueryConfig = {
  readonly queryKey: ReturnType<typeof notificationKeys.list>;
  readonly queryFn: (context: {
    readonly pageParam?: string;
  }) => Promise<NotificationsPageData>;
  readonly initialPageParam: string | undefined;
  readonly getNextPageParam: typeof keysetNextPageParam;
};


/** Опции ленты уведомлений (канон #887): один источник ключ+fetch для
 * хука и серверного префетча. */
export function notificationsFeedQueryOptions({
  unreadOnly,
  transport,
}: {
  readonly unreadOnly: boolean;
  readonly transport?: ApiTransport;
}): NotificationsFeedQueryConfig {
  return {
    queryKey: notificationKeys.list(unreadOnly),
    queryFn: ({ pageParam }: { pageParam?: string }) =>
      fetchNotificationsPage({ unreadOnly, cursor: pageParam, transport }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: keysetNextPageParam,
  };
}


/** Чистый fetch счётчика непрочитанных — общее горло хука и серверного
 * префетча #887. */
export async function fetchUnreadNotificationsCount(
  transport: ApiTransport = apiClient,
): Promise<number> {
  const response = await transport<UnreadCountDto>('/notifications/unread-count');
  return response.count;
}


/** Опции счётчика непрочитанных (канон #887): один источник ключ+fetch
 * для хука и серверного префетча. */
export function unreadNotificationsCountQueryOptions(
  transport: ApiTransport = apiClient,
): UseQueryOptions<number, ApiError, number, ReturnType<typeof notificationKeys.unreadCount>> {
  return queryOptions({
    queryKey: notificationKeys.unreadCount(),
    queryFn: () => fetchUnreadNotificationsCount(transport),
  });
}


/** Чистый fetch страницы уведомления — общее горло хука и серверного
 * префетча #887. */
export async function fetchNotificationDetail(
  id: string,
  transport: ApiTransport = apiClient,
): Promise<NotificationDetail> {
  const response = await transport<NotificationDetailDto>(`/notifications/${id}`);
  return mapNotificationDetail(response);
}


/** Опции страницы уведомления (канон #887): один источник ключ+fetch для
 * хука и серверного префетча; живые действия требуют свежести — staleTime
 * остаётся на потребителе. */
export function notificationDetailQueryOptions({
  id,
  transport = apiClient,
}: {
  readonly id: string;
  readonly transport?: ApiTransport;
}): UseQueryOptions<NotificationDetail, ApiError, NotificationDetail, ReturnType<typeof notificationKeys.detail>> {
  return queryOptions({
    queryKey: notificationKeys.detail(id),
    queryFn: () => fetchNotificationDetail(id, transport),
  });
}

