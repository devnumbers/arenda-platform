import { describe, expect, it } from 'vitest';
import type { Task } from '@/entities/task';
import {
  canMutateFeedTask,
  mutableFeedTasks,
  taskCompletionPath,
  type FeedPropertyRef,
} from './global-tasks';

function task(partial: Partial<Task> & { readonly id: string }): Task {
  return {
    propertyId: null,
    ruleId: 'r1',
    dueDate: null,
    dueTime: null,
    title: 'Задача',
    comment: null,
    repeat: null,
    completedDate: null,
    status: 'active',
    createdAt: '2026-09-01T10:00:00Z',
    updatedAt: '2026-09-01T10:00:00Z',
    ...partial,
  };
}

const properties: Record<string, FeedPropertyRef> = {
  own: { role: 'owner', status: 'active' },
  shared: { role: 'full_access', status: 'active' },
  watched: { role: 'viewer', status: 'active' },
  archived: { role: 'owner', status: 'archived' },
};
const propertyOf = (propertyId: string): FeedPropertyRef | undefined =>
  properties[propertyId];

describe('taskCompletionPath — маршрут мутации выполнения по срезу (ADR 0052)', () => {
  it('безобъектная задача уходит в глобальный путь /tasks', () => {
    expect(taskCompletionPath(task({ id: 't1', propertyId: null }), 'complete')).toBe(
      '/tasks/t1/complete',
    );
  });

  it('объектная задача уходит в путь объекта', () => {
    expect(
      taskCompletionPath(task({ id: 't2', propertyId: 'p1' }), 'uncomplete'),
    ).toBe('/properties/p1/tasks/t2/uncomplete');
  });

  it('кодирует идентификаторы в URL', () => {
    expect(
      taskCompletionPath(task({ id: 't/x', propertyId: 'p/y' }), 'complete'),
    ).toBe('/properties/p%2Fy/tasks/t%2Fx/complete');
  });
});

describe('canMutateFeedTask — мутабельность строки ленты', () => {
  it('безобъектная задача — книга самого читателя, мутабельна', () => {
    expect(canMutateFeedTask(task({ id: 't1', propertyId: null }), propertyOf)).toBe(
      true,
    );
  });

  it('объектная задача: владелец и полный доступ — мутабельны', () => {
    expect(canMutateFeedTask(task({ id: 't2', propertyId: 'own' }), propertyOf)).toBe(
      true,
    );
    expect(
      canMutateFeedTask(task({ id: 't3', propertyId: 'shared' }), propertyOf),
    ).toBe(true);
  });

  it('объектная задача: зритель только читает (ADR 0028)', () => {
    expect(
      canMutateFeedTask(task({ id: 't4', propertyId: 'watched' }), propertyOf),
    ).toBe(false);
  });

  it('архивный объект гасит мутации (#446)', () => {
    expect(
      canMutateFeedTask(task({ id: 't5', propertyId: 'archived' }), propertyOf),
    ).toBe(false);
  });

  it('объект без записи в справочнике — страховка: только чтение', () => {
    expect(
      canMutateFeedTask(task({ id: 't6', propertyId: 'unknown' }), propertyOf),
    ).toBe(false);
  });
});

describe('mutableFeedTasks — «Отметить все» касается только своих строк', () => {
  it('оставляет безобъектные и объекты с правом мутации', () => {
    const feed = [
      task({ id: 't1', propertyId: null }),
      task({ id: 't2', propertyId: 'watched' }),
      task({ id: 't3', propertyId: 'shared' }),
      task({ id: 't4', propertyId: 'archived' }),
    ];
    expect(mutableFeedTasks(feed, propertyOf).map((t) => t.id)).toEqual([
      't1',
      't3',
    ]);
  });
});
