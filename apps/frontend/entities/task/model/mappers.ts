/**
 * DTO → entity модели слайса задач. Контракт camelCase; маппер только
 * переносит поля — нормализовать нечего, nullables уже совпадают с
 * доменной моделью.
 */

import type { components } from '@/shared/api/dto';
import type { Task, TaskRule, TasksPage } from './types';

type TaskDto = components['schemas']['TaskResponse'];
type TaskRuleDto = components['schemas']['TaskRuleResponse'];
type TasksPageDto = components['schemas']['TasksResponse'];

export function mapTask(dto: TaskDto): Task {
  return {
    id: dto.id,
    propertyId: dto.propertyId,
    ruleId: dto.ruleId,
    dueDate: dto.dueDate,
    dueTime: dto.dueTime,
    title: dto.title,
    comment: dto.comment,
    repeat: dto.repeat,
    completedDate: dto.completedDate,
    status: dto.status,
    createdAt: dto.createdAt,
    updatedAt: dto.updatedAt,
  };
}

export function mapTaskRule(dto: TaskRuleDto): TaskRule {
  return {
    id: dto.id,
    propertyId: dto.propertyId,
    title: dto.title,
    comment: dto.comment,
    dueDate: dto.dueDate,
    dueTime: dto.dueTime,
    repeat: dto.repeat,
    createdAt: dto.createdAt,
    updatedAt: dto.updatedAt,
  };
}

export function mapTasksPage(dto: TasksPageDto): TasksPage {
  return {
    items: dto.items.map(mapTask),
    total: dto.total,
    today: dto.today,
  };
}
