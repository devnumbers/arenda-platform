import { describe, expect, it } from 'vitest';

import { EMPTY_TASKS_FEED_FILTER } from '@/features/tasks';

import { tasksFeedEmptyState } from './tasks-feed-gate';

const onePropertyFilter = { propertyIds: ['00000000-0000-0000-0000-000000000001'], withoutProperty: false };
const withoutPropertyFilter = { propertyIds: [], withoutProperty: true };

describe('tasksFeedEmptyState', () => {
  it('всё загружено, задач нет, фильтр пуст — полное пустое состояние ленты', () => {
    expect(
      tasksFeedEmptyState({ filter: EMPTY_TASKS_FEED_FILTER, loaded: true, hasTasks: false }),
    ).toBe('feed');
  });

  it('всё загружено, задач нет, выбран объект — пусто с фильтром, чипы остаются', () => {
    expect(
      tasksFeedEmptyState({ filter: onePropertyFilter, loaded: true, hasTasks: false }),
    ).toBe('filtered');
  });

  it('всё загружено, задач нет, только «Общие задачи» — тоже пусто с фильтром', () => {
    expect(
      tasksFeedEmptyState({ filter: withoutPropertyFilter, loaded: true, hasTasks: false }),
    ).toBe('filtered');
  });

  it('есть задачи — обычная лента, даже под фильтром', () => {
    expect(
      tasksFeedEmptyState({ filter: onePropertyFilter, loaded: true, hasTasks: true }),
    ).toBe('none');
  });

  it('выборки ещё грузятся — пустоту не утверждаем, экран показывает скелетон', () => {
    expect(
      tasksFeedEmptyState({ filter: EMPTY_TASKS_FEED_FILTER, loaded: false, hasTasks: false }),
    ).toBe('none');
  });
});
