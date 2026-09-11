import { describe, expect, it } from 'vitest';

import type { Task, TasksPage } from '@/entities/task';

import { propertyTasksSummary } from './tasks-summary';

function taskFixture(overrides: Partial<Task> = {}): Task {
  return {
    id: 'task-1',
    propertyId: 'property-1',
    propertyName: null,
    ruleId: 'rule-1',
    dueDate: '2026-09-11',
    dueTime: null,
    title: 'Позвонить мастеру',
    comment: null,
    repeat: null,
    completedDate: null,
    status: 'active',
    createdAt: '2026-09-01T00:00:00Z',
    updatedAt: '2026-09-01T00:00:00Z',
    ...overrides,
  };
}

function pageFixture(items: ReadonlyArray<Task>, total = items.length): TasksPage {
  return { items, total, today: '2026-09-11' };
}

describe('propertyTasksSummary', () => {
  it('сводка активных задач с числом просроченных', () => {
    const page = pageFixture([
      taskFixture({ id: 't1', status: 'overdue', dueDate: '2026-09-01' }),
      taskFixture({ id: 't2' }),
      taskFixture({ id: 't3' }),
    ]);
    expect(propertyTasksSummary(page)).toEqual({
      title: '3 активные задачи',
      overdueCount: 1,
    });
  });

  it('согласование: 1 активная задача, 5 активных задач, 21 активная задача', () => {
    const title = (count: number): string =>
      propertyTasksSummary(
        pageFixture(
          Array.from({ length: count }, (_, index) =>
            taskFixture({ id: `t${index}` }),
          ),
        ),
      )?.title ?? '';
    expect(title(1)).toBe('1 активная задача');
    expect(title(5)).toBe('5 активных задач');
    expect(title(21)).toBe('21 активная задача');
  });

  it('без просроченных — ноль', () => {
    const page = pageFixture([taskFixture({ id: 't1' })]);
    expect(propertyTasksSummary(page)?.overdueCount).toBe(0);
  });

  it('пустая страница — сводки нет', () => {
    expect(propertyTasksSummary(pageFixture([]))).toBeNull();
  });

  it('заголовок по серверному total, просроченные — по странице', () => {
    const page = pageFixture(
      [taskFixture({ id: 't1', status: 'overdue', dueDate: '2026-09-01' })],
      501,
    );
    expect(propertyTasksSummary(page)).toEqual({
      title: '501 активная задача',
      overdueCount: 1,
    });
  });
});
