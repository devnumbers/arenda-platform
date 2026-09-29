import { queryOptions, type UseQueryOptions } from '@tanstack/react-query';
import { apiClient, type ApiTransport } from '@/shared/api/client';
import type { ApiError } from '@/shared/api/errors';
import { mapTasksPage } from '@/entities/task';
import type { TasksPage } from '@/entities/task';
import { taskKeys } from '@/shared/api/query-keys';
import type { components } from '@/shared/api/dto';
type TasksPageDto = components['schemas']['TasksResponse'];

/** Лимит листинга: экран группирует весь список целиком, поэтому берёт
 * максимум контракта одной страницей (серверный дефолт — 100, максимум
 * 500); порции с догрузкой — если у объекта появится больше задач.
 * Единственный экземпляр: хуки ленты импортируют его отсюда. */
export const TASKS_PAGE_LIMIT = 500;

/** Двойной модуль API-слоя tasks (без 'use client'): чистые fetch-функции
 * и queryOptions-фабрики канона #887 — общий источник ключ+fetch для
 * клиентских хуков, прогрева хабов #626 и серверного префетча. Хуки — в
 * hooks.ts ('use client'). */

/** Путь активного/выполненного бакета глобального листинга GET /tasks:
 * фильтр (#524/#547) — необязательный comma-separated сегмент propertyId и
 * флаг withoutProperty («Общие задачи», union с объектами — решение
 * владельца 2026-09-07) перед бакетом. */
function globalTasksPath(
  completed: boolean,
  propertyIds: ReadonlyArray<string>,
  withoutProperty: boolean,
): string {
  const parts: string[] = [];
  if (propertyIds.length > 0) {
    parts.push(`propertyId=${encodeURIComponent(propertyIds.join(','))}`);
  }
  if (withoutProperty) {
    parts.push('withoutProperty=true');
  }
  parts.push(`completed=${completed}`, `limit=${TASKS_PAGE_LIMIT}`);
  return `/tasks?${parts.join('&')}`;
}

/** Чистый fetch бакета глобального листинга GET /tasks — общее горло хуков
 * ленты и прогрева хабов #626: прогрев кэша идёт тем же кодом, что
 * читает экран (детали среза — у useGlobalActiveTasks ниже). */
export function fetchGlobalTasks(
  completed: boolean,
  propertyIds: ReadonlyArray<string>,
  withoutProperty: boolean,
  transport: ApiTransport = apiClient,
): Promise<TasksPage> {
  return transport<TasksPageDto>(globalTasksPath(completed, propertyIds, withoutProperty))
    .then(mapTasksPage);
}


/** Чистый fetch активных задач объекта — общее горло хука и серверного
 * префетча #887. */
export async function fetchActiveTasks(
  propertyId: string,
  transport: ApiTransport = apiClient,
): Promise<TasksPage> {
  const response = await transport<TasksPageDto>(
    `/properties/${encodeURIComponent(propertyId)}/tasks?completed=false&limit=${TASKS_PAGE_LIMIT}`,
  );
  return mapTasksPage(response);
}

/** Опции активных задач объекта (канон #887): один источник ключ+fetch
 * для хука и серверного префетча. */
export function activeTasksQueryOptions({
  propertyId,
  transport = apiClient,
}: {
  readonly propertyId: string;
  readonly transport?: ApiTransport;
}): UseQueryOptions<TasksPage, ApiError, TasksPage, ReturnType<typeof taskKeys.active>> {
  return queryOptions({
    queryKey: taskKeys.active(propertyId),
    queryFn: () => fetchActiveTasks(propertyId, transport),
  });
}


/** Опции бакета глобальной ленты (канон #887): один источник ключ+fetch
 * для хуков, прогрева хабов #626 и серверного префетча. */
export function globalTasksQueryOptions({
  completed,
  propertyIds,
  withoutProperty,
  transport = apiClient,
}: {
  readonly completed: boolean;
  readonly propertyIds: ReadonlyArray<string>;
  readonly withoutProperty: boolean;
  readonly transport?: ApiTransport;
}): UseQueryOptions<TasksPage, ApiError, TasksPage, ReturnType<typeof taskKeys.global>> {
  return queryOptions({
    queryKey: taskKeys.global(completed, propertyIds, withoutProperty),
    queryFn: () => fetchGlobalTasks(completed, propertyIds, withoutProperty, transport),
  });
}

