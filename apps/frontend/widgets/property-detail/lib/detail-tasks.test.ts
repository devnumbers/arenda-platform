import { describe, expect, it } from 'vitest';

import type { Task, TasksPage } from '@/entities/task';

import { propertyDetailTaskTone, propertyDetailTasks } from './detail-tasks';

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

function pageFixture(items: ReadonlyArray<Task>): TasksPage {
  return { items, total: items.length, today: '2026-09-11' };
}

describe('propertyDetailTasks', () => {
  it('максимум 3 задачи: сначала просрочки от старейшей, затем по ближайшей дате', () => {
    const page = pageFixture([
      taskFixture({ id: 'far', dueDate: '2026-12-01' }),
      taskFixture({ id: 'overdue-new', dueDate: '2026-09-05', status: 'overdue' }),
      taskFixture({ id: 'soon', dueDate: '2026-09-12' }),
      taskFixture({ id: 'overdue-old', dueDate: '2026-09-01', status: 'overdue' }),
      taskFixture({ id: 'mid', dueDate: '2026-10-01' }),
    ]);
    expect(propertyDetailTasks(page).map((task) => task.id)).toEqual([
      'overdue-old',
      'overdue-new',
      'soon',
    ]);
  });

  it('недатированные задачи не выводятся (решение владельца 11.09)', () => {
    const page = pageFixture([
      taskFixture({ id: 'undated', dueDate: null, status: 'undated' }),
      taskFixture({ id: 'dated', dueDate: '2026-09-15' }),
    ]);
    expect(propertyDetailTasks(page).map((task) => task.id)).toEqual(['dated']);
  });

  it('меньше трёх — сколько есть', () => {
    const page = pageFixture([taskFixture({ id: 'only' })]);
    expect(propertyDetailTasks(page).map((task) => task.id)).toEqual(['only']);
  });

  it('пустая страница — пусто', () => {
    expect(propertyDetailTasks(pageFixture([]))).toEqual([]);
  });
});

describe('propertyDetailTaskTone', () => {
  it('просрочка красная, сегодня/завтра синие, дальше серые', () => {
    const today = '2026-09-11';
    expect(propertyDetailTaskTone(taskFixture({ status: 'overdue' }), today)).toBe('danger');
    expect(propertyDetailTaskTone(taskFixture({ dueDate: '2026-09-11' }), today)).toBe('primary');
    expect(propertyDetailTaskTone(taskFixture({ dueDate: '2026-09-12' }), today)).toBe('primary');
    expect(propertyDetailTaskTone(taskFixture({ dueDate: '2026-10-01' }), today)).toBe('muted');
  });
});
