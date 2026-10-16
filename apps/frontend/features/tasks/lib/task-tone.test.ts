import { describe, expect, it } from 'vitest';

import type { Task } from '@/entities/task';

import { taskRowTone, taskSectionTone } from './task-tone';

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

describe('taskRowTone (канон строк задач)', () => {
  const today = '2026-09-11';

  it('просрочка красная', () => {
    expect(taskRowTone(taskFixture({ status: 'overdue' }), today)).toBe('danger');
  });

  it('сегодня и завтра — синие', () => {
    expect(taskRowTone(taskFixture({ dueDate: '2026-09-11' }), today)).toBe('primary');
    expect(taskRowTone(taskFixture({ dueDate: '2026-09-12' }), today)).toBe('primary');
  });

  it('дальше и без даты — серые', () => {
    expect(taskRowTone(taskFixture({ dueDate: '2026-10-01' }), today)).toBe('muted');
    expect(taskRowTone(taskFixture({ dueDate: null }), today)).toBe('muted');
  });
});

describe('taskSectionTone (секции экранов задач)', () => {
  it('просрочка красная, сегодня/завтра синие, остальные серые', () => {
    expect(taskSectionTone('overdue')).toBe('danger');
    expect(taskSectionTone('today')).toBe('primary');
    expect(taskSectionTone('tomorrow')).toBe('primary');
    expect(taskSectionTone('dated')).toBe('muted');
    expect(taskSectionTone('undated')).toBe('muted');
    expect(taskSectionTone('completed')).toBe('muted');
  });
});
