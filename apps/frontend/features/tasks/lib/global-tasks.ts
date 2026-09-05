/**
 * Модель глобальной ленты задач (карта #518, тикет #523): строки двух срезов
 * — объектного и безобъектного (ADR 0052) — сходятся на одном экране, а
 * мутации выполнения у срезов разные пути. Маршрутизация по срезу и
 * мутабельность строки считаются здесь, чтобы экран и хуки оставались
 * тонкими.
 */

import type { PropertyStatus } from '@/entities/property';
import type { AccessRole } from '@/shared/model/access';
import type { Task } from '@/entities/task';

/** Действие над фактом выполнения задачи. */
export type TaskCompletionAction = 'complete' | 'uncomplete';

/** Путь мутации выполнения: безобъектная задача живёт в книге владельца и
 * идёт в глобальный путь /tasks (ADR 0052, #520), объектная — в путь своего
 * объекта. Журнал удалённого правила маршрут не меняет — 409 гасит UI
 * (кружок журнала статичен). */
export function taskCompletionPath(
  task: Task,
  action: TaskCompletionAction,
): string {
  const verb = `/${encodeURIComponent(task.id)}/${action}`;
  if (task.propertyId === null) {
    return `/tasks${verb}`;
  }
  return `/properties/${encodeURIComponent(task.propertyId)}/tasks${verb}`;
}

/** Роль и статус объекта из справочника объектов — то, что лента знает о
 * свойстве строки (useProperties). */
export type FeedPropertyRef = {
  readonly role?: AccessRole;
  readonly status?: PropertyStatus;
};

/** Мутабельность строки ленты (ADR 0028, решения #522): безобъектная задача
 * в ленте всегда своя (срез owner-only — если видна, то читателю), объектная
 * — по роли из справочника: зритель читает, остальные меняют; архивный
 * объект гасит мутации (#446). Объект без записи в справочнике — страховка
 * «только чтение»: сервер всё равно ответил бы отказом. */
export function canMutateFeedTask(
  task: Task,
  propertyOf: (propertyId: string) => FeedPropertyRef | undefined,
): boolean {
  if (task.propertyId === null) {
    return true;
  }
  const property = propertyOf(task.propertyId);
  if (property === undefined) {
    return false;
  }
  return property.role !== 'viewer' && property.status !== 'archived';
}

/** Подмножество ленты, доступное мутациям, — адресат «Отметить все»
 * (клиентский цикл по одной POST на задачу, решение владельца 2026-09-03). */
export function mutableFeedTasks(
  tasks: readonly Task[],
  propertyOf: (propertyId: string) => FeedPropertyRef | undefined,
): readonly Task[] {
  return tasks.filter((task) => canMutateFeedTask(task, propertyOf));
}
