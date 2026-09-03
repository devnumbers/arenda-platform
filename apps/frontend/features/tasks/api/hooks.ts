'use client';

import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import type { ApiError } from '@/shared/api/errors';
import { mapTask, mapTasksPage } from '@/entities/task';
import type { Task, TasksPage } from '@/entities/task';
import { taskKeys } from '@/shared/api/query-keys';
import type { components } from '@/shared/api/dto';

type TaskResponseDto = components['schemas']['TaskResponse'];
type TasksPageDto = components['schemas']['TasksResponse'];

/** Лимит листинга: экран группирует весь список целиком, поэтому берёт
 * максимум контракта одной страницей (серверный дефолт — 100, максимум
 * 500); порции с догрузкой — если у объекта появится больше задач. */
const TASKS_PAGE_LIMIT = 500;

/**
 * Активные задачи объекта — сырьё секций «Просроченные / Сегодня / Завтра /
 * даты / Без даты». Просрочка и «без срока» — серверно-вычисляемые статусы,
 * «сегодня» приходит в ответе по TZ собственника (ADR 0048) — клиент зоны
 * не знает.
 */
export function useActiveTasks(
  propertyId: string,
): UseQueryResult<TasksPage, ApiError> {
  return useQuery({
    queryKey: taskKeys.active(propertyId),
    queryFn: async () => {
      const response = await apiClient<TasksPageDto>(
        `/properties/${encodeURIComponent(propertyId)}/tasks?completed=false&limit=${TASKS_PAGE_LIMIT}`,
      );
      return mapTasksPage(response);
    },
    enabled: Boolean(propertyId),
  });
}

/**
 * Журнал выполненных — сворачиваемая секция «Выполненные N»; total нужен
 * счётчиком секции (может быть больше страницы). Секция скрыта, пока
 * журнал пуст.
 */
export function useCompletedTasks(
  propertyId: string,
): UseQueryResult<TasksPage, ApiError> {
  return useQuery({
    queryKey: taskKeys.completed(propertyId),
    queryFn: async () => {
      const response = await apiClient<TasksPageDto>(
        `/properties/${encodeURIComponent(propertyId)}/tasks?completed=true&limit=${TASKS_PAGE_LIMIT}`,
      );
      return mapTasksPage(response);
    },
    enabled: Boolean(propertyId),
  });
}

/**
 * Выполнить задачу (тап по кружку): факт датируется сегодня в TZ
 * собственника; повторное выполнение — контрактный 409. Обратимо, пока
 * правило живо (словарь #494). Уведомлений об успехе нет (решение
 * владельца) — задача просто переезжает в «Выполненные».
 */
export function useCompleteTask(
  propertyId: string,
): UseMutationResult<Task, ApiError, string> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (taskId: string) => {
      const response = await apiClient<TaskResponseDto>(
        `/properties/${encodeURIComponent(propertyId)}`
          + `/tasks/${encodeURIComponent(taskId)}/complete`,
        { method: 'POST' },
      );
      return mapTask(response);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: taskKeys.all });
    },
  });
}

/**
 * Снять выполнение (тап по кружку выполненной): возвращается в активные;
 * у журнала удалённого правила — контрактный 409.
 */
export function useUncompleteTask(
  propertyId: string,
): UseMutationResult<Task, ApiError, string> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (taskId: string) => {
      const response = await apiClient<TaskResponseDto>(
        `/properties/${encodeURIComponent(propertyId)}`
          + `/tasks/${encodeURIComponent(taskId)}/uncomplete`,
        { method: 'POST' },
      );
      return mapTask(response);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: taskKeys.all });
    },
  });
}

/**
 * «Отметить все задачи» (меню кебаба #499, Figma 1535-77633): выполняет
 * все активные задачи объекта по одному POST /complete на задачу —
 * bulk-эндпоинта в контракте нет, задач у объекта обычно единицы. Сбои
 * отдельных запросов (в т.ч. контрактный 409 на уже выполненную) глотаются
 * осознанно — никаких уведомлений, список перечитается и покажет факт
 * (решение владельца 2026-09-03). Инвалидация одна, на весь прогон.
 */
export function useCompleteAllTasks(
  propertyId: string,
): UseMutationResult<void, ApiError, readonly string[]> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (taskIds: readonly string[]) => {
      await Promise.allSettled(
        taskIds.map((taskId) =>
          apiClient<TaskResponseDto>(
            `/properties/${encodeURIComponent(propertyId)}`
              + `/tasks/${encodeURIComponent(taskId)}/complete`,
            { method: 'POST' },
          ),
        ),
      );
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: taskKeys.all });
    },
  });
}

/**
 * «Удалить все выполненные» (кебаб ⋮ → шит подтверждения): скоуп — журнал
 * удалённых правил (ADR 0051 §3), выполненные живых правил остаются
 * (держат дедуп-ключи тика). Счётчик секции после операции перечитается.
 */
export function useDeleteCompletedTasks(
  propertyId: string,
): UseMutationResult<void, ApiError, void> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      await apiClient<void>(
        `/properties/${encodeURIComponent(propertyId)}/tasks/completed`,
        { method: 'DELETE' },
      );
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: taskKeys.all });
    },
  });
}
