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
import { mapTask, mapTaskRule, mapTasksPage } from '@/entities/task';
import type { Task, TaskRule, TasksPage } from '@/entities/task';
import { taskKeys } from '@/shared/api/query-keys';
import type { components } from '@/shared/api/dto';
import { buildTaskRuleCreateRequest, taskRuleCreatePath, type TaskCreateDraft } from '../lib/task-create';
import { taskCompletionPath } from '../lib/global-tasks';
import type { TaskRuleUpdateCommand } from '../lib/task-edit';

type TaskResponseDto = components['schemas']['TaskResponse'];
type TaskRuleResponseDto = components['schemas']['TaskRuleResponse'];
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

/**
 * Создание задачи = создание правила (словарь #494, #500): POST правил
 * сразу материализует вхождения. Дата опциональна — без неё задача попадает
 * в «Без срока»; repeat в контракте обязателен, «без повтора» уходит как
 * once (черновик собирает lib/task-create). Эндпоинт выбирает объект
 * черновика (#525): с объектом — объектный путь, «Общая задача» —
 * глобальный /tasks/rules (#520). Тело ответа экрану не нужно —
 * результат виден по перечитанному списку (инвалидация всего taskKeys,
 * как у остальных мутаций контекста).
 */
export function useCreateTaskRule(): UseMutationResult<void, ApiError, TaskCreateDraft> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (draft: TaskCreateDraft) => {
      const request = buildTaskRuleCreateRequest(draft);
      if (request === null) {
        // Недостижимо через UI: кнопка «Создать» задизейблена canCreateTask.
        throw new Error('Черновик задачи не прошёл валидацию');
      }
      await apiClient<unknown>(taskRuleCreatePath(draft.propertyId), {
        method: 'POST',
        body: JSON.stringify(request),
      });
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: taskKeys.all });
    },
  });
}

/** Правило задачи — предзаполнение формы «Изменить задачу» (#502). */
export function useTaskRule(
  propertyId: string,
  ruleId: string,
): UseQueryResult<TaskRule, ApiError> {
  return useQuery({
    queryKey: taskKeys.rule(propertyId, ruleId),
    queryFn: async () => {
      const response = await apiClient<TaskRuleResponseDto>(
        `/properties/${encodeURIComponent(propertyId)}/tasks/rules/${encodeURIComponent(ruleId)}`,
      );
      return mapTaskRule(response);
    },
    enabled: Boolean(propertyId) && Boolean(ruleId),
  });
}

/**
 * Правило без объекта — предзаполнение плоской формы «Изменить задачу»
 * (#537): глобальный GET /tasks/rules/{ruleId} (#520), owner-only. Правило,
 * привязанное к объекту, на этом пути невидимо — privacy 404, срезы не
 * смешиваются (ADR 0052).
 */
export function usePropertylessTaskRule(
  ruleId: string,
): UseQueryResult<TaskRule, ApiError> {
  return useQuery({
    queryKey: taskKeys.propertylessRule(ruleId),
    queryFn: async () => {
      const response = await apiClient<TaskRuleResponseDto>(
        `/tasks/rules/${encodeURIComponent(ruleId)}`,
      );
      return mapTaskRule(response);
    },
    enabled: Boolean(ruleId),
  });
}

/**
 * «Сегодня» владельца для плоской формы правки (ADR 0048) — страница
 * глобального листинга GET /tasks?withoutProperty=true (#521); лимит 1:
 * форме нужна только дата, ленту безобъектного среза строит экран #523.
 * Владелец — сам читатель, today его календаря (контракт #521). enabled —
 * для форм, где срез входа другой (создание с объекта, #525).
 */
export function usePropertylessTasks(
  options: { readonly enabled?: boolean } = {},
): UseQueryResult<TasksPage, ApiError> {
  return useQuery({
    queryKey: taskKeys.propertylessTasks(),
    queryFn: async () => {
      const response = await apiClient<TasksPageDto>(
        '/tasks?withoutProperty=true&completed=false&limit=1',
      );
      return mapTasksPage(response);
    },
    enabled: options.enabled ?? true,
  });
}

/** Путь активного/выполненного бакета глобального листинга GET /tasks:
 * фильтр объектов (#524/#547) — необязательный comma-separated сегмент
 * перед бакетом (контракт propertyId, мультивыбор — #547). */
const globalTasksPath = (completed: boolean, propertyIds: ReadonlyArray<string>): string => {
  const scope =
    propertyIds.length === 0 ? '' : `propertyId=${encodeURIComponent(propertyIds.join(','))}&`;
  return `/tasks?${scope}completed=${completed}&limit=${TASKS_PAGE_LIMIT}`;
};

/**
 * Активный бакет глобальной ленты GET /tasks (#521, экран #523):
 * merged-фид читателя (свои задачи + задачи видимых объектов, архивы мимо —
 * решения #522); фильтр объектов (#524/#547) — срез перечисленных
 * объектов, смена фильтра меняет ключ и перечитывает. Лимит — максимум
 * контракта: лента группируется целиком, как на объекте. today — календарь
 * читателя (на срезе одного объекта — владельца данных). Сестринский хук
 * журнала — useGlobalCompletedTasks.
 */
export function useGlobalActiveTasks(
  propertyIds: ReadonlyArray<string>,
): UseQueryResult<TasksPage, ApiError> {
  return useQuery({
    queryKey: taskKeys.global(false, propertyIds),
    queryFn: async () =>
      mapTasksPage(await apiClient<TasksPageDto>(globalTasksPath(false, propertyIds))),
  });
}

/** Журнал глобальной ленты — сворачиваемая секция «Выполненные N». */
export function useGlobalCompletedTasks(
  propertyIds: ReadonlyArray<string>,
): UseQueryResult<TasksPage, ApiError> {
  return useQuery({
    queryKey: taskKeys.global(true, propertyIds),
    queryFn: async () =>
      mapTasksPage(await apiClient<TasksPageDto>(globalTasksPath(true, propertyIds))),
  });
}

/**
 * Выполнить/снять в ленте: срез маршрутизирует lib/global-tasks (ADR 0052)
 * — безобъектная задача в глобальный путь, объектная в путь своего объекта.
 * Инвалидация taskKeys.all перечитывает и ленту, и все объектные экраны.
 */
export function useCompleteGlobalTask(
  action: 'complete' | 'uncomplete',
): UseMutationResult<Task, ApiError, Task> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (task: Task) => {
      const response = await apiClient<TaskResponseDto>(
        taskCompletionPath(task, action),
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
 * «Отметить все» в ленте: по одному POST на задачу (bulk-эндпоинта нет),
 * маршрут — по срезу каждой строки. Цикл последовательный: параллельные
 * POST complete ловят серверный дедлок на блокировке строк свойств
 * владельца (SQLSTATE 40P01 — дефект сериализации, отдельный тикет бэка).
 * Сбои отдельных запросов глотаются осознанно, как на объекте (решение
 * владельца 2026-09-03) — список перечитается и покажет факт. Выбор
 * мутабельных строк — lib/global-tasks.
 */
export function useCompleteAllGlobalTasks(): UseMutationResult<
  void,
  ApiError,
  readonly Task[]
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (tasks: readonly Task[]) => {
      for (const task of tasks) {
        try {
          await apiClient<TaskResponseDto>(taskCompletionPath(task, 'complete'), {
            method: 'POST',
          });
        } catch {
          // Проглатываем: факт покажет перечитанный список.
        }
      }
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: taskKeys.all });
    },
  });
}

/**
 * «Удалить выполненные» ленты: глобальный DELETE /tasks/completed (#536) —
 * журнал удалённых правил всей книги читателя (owner-scope ADR 0028):
 * журналы общих объектов чужие, их сносит владелец на своём экране.
 */
export function useDeleteCompletedGlobalTasks(): UseMutationResult<
  void,
  ApiError,
  void
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      await apiClient<void>('/tasks/completed', { method: 'DELETE' });
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: taskKeys.all });
    },
  });
}

/**
 * Сохранение правки правила (#502): частичный PATCH — команду-дифф собирает
 * lib/task-edit (только изменённые поля; comment/dueDate/dueTime —
 * три-стейт, null очищает). Сервер перематериализует будущие вхождения
 * свежими снимками, выполненные остаются — список и журнал перечитываются
 * инвалидацией taskKeys.
 */
export function useUpdateTaskRule(
  propertyId: string,
  ruleId: string,
): UseMutationResult<void, ApiError, TaskRuleUpdateCommand> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (command: TaskRuleUpdateCommand) => {
      await apiClient<unknown>(
        `/properties/${encodeURIComponent(propertyId)}/tasks/rules/${encodeURIComponent(ruleId)}`,
        { method: 'PATCH', body: JSON.stringify(command) },
      );
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: taskKeys.all });
    },
  });
}

/**
 * Сохранение правки правила без объекта (#537): тот же частичный PATCH,
 * что у объектного пути (#502), — глобальный /tasks/rules/{ruleId} (#520).
 * Команду-дифф собирает общий lib/task-edit; сервер перематериализует
 * будущие вхождения по поясу владельца (ADR 0052). Ответа экрану не нужно —
 * инвалидация taskKeys.all перечитывает и объектный, и безобъектный срезы.
 */
export function useUpdatePropertylessTaskRule(
  ruleId: string,
): UseMutationResult<void, ApiError, TaskRuleUpdateCommand> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (command: TaskRuleUpdateCommand) => {
      await apiClient<unknown>(
        `/tasks/rules/${encodeURIComponent(ruleId)}`,
        { method: 'PATCH', body: JSON.stringify(command) },
      );
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: taskKeys.all });
    },
  });
}
