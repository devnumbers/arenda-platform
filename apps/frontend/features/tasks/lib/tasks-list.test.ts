import { describe, expect, it } from 'vitest';
import type { Task } from '@/entities/task';
import { addDays } from '@/entities/task';
import {
  DEFAULT_TASKS_SORT,
  groupTasks,
  parseTasksSortParams,
  serializeTasksSortToParams,
  sortTasks,
  type TasksSort,
} from './tasks-list';

const TODAY = '2026-09-10';

function task(partial: Partial<Task> & { readonly id: string }): Task {
  return {
    propertyId: 'p1',
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

describe('groupTasks — порядок секций', () => {
  it('выстраивает Просроченные → Сегодня → Завтра → даты → Без даты → Выполненные', () => {
    const active = [
      task({ id: '1', dueDate: addDays(TODAY, 5), status: 'active' }),
      task({ id: '2', dueDate: addDays(TODAY, -9), status: 'overdue' }),
      task({ id: '3', status: 'undated' }),
      task({ id: '4', dueDate: TODAY, status: 'active' }),
      task({ id: '5', dueDate: addDays(TODAY, -2), status: 'overdue' }),
      task({ id: '6', dueDate: addDays(TODAY, 1), status: 'active' }),
      task({ id: '7', dueDate: addDays(TODAY, 2), status: 'active' }),
    ];
    const completed = [task({ id: '8', status: 'completed', completedDate: TODAY })];

    const sections = groupTasks(active, completed, TODAY, DEFAULT_TASKS_SORT);

    expect(sections.map((section) => section.kind)).toEqual([
      'overdue',
      'today',
      'tomorrow',
      'dated',
      'dated',
      'undated',
      'completed',
    ]);
    expect(sections.map((section) => section.tasks.map((t) => t.id))).toEqual([
      ['2', '5'],
      ['4'],
      ['6'],
      ['7'],
      ['1'],
      ['3'],
      ['8'],
    ]);
  });

  it('пропускает пустые секции — остаётся только датированная', () => {
    const active = [task({ id: '1', dueDate: addDays(TODAY, 3) })];
    const sections = groupTasks(active, [], TODAY, DEFAULT_TASKS_SORT);
    expect(sections).toHaveLength(1);
    expect(sections[0]?.kind).toBe('dated');
    expect(sections[0]?.date).toBe(addDays(TODAY, 3));
  });

  it('датированные секции идут по возрастанию даты независимо от направления сортировки', () => {
    const active = [
      task({ id: 'late', dueDate: addDays(TODAY, 9) }),
      task({ id: 'early', dueDate: addDays(TODAY, 4) }),
    ];
    const desc: TasksSort = { field: 'date', direction: 'desc' };
    const sections = groupTasks(active, [], TODAY, desc);
    expect(sections.map((section) => section.tasks.map((t) => t.id))).toEqual([
      ['early'],
      ['late'],
    ]);
  });
});

describe('groupTasks — заголовки секций', () => {
  it('называет служебные секции и датирует Сегодня/Завтра', () => {
    const active = [
      task({ id: '1', dueDate: TODAY, status: 'overdue' }),
      task({ id: '2', dueDate: TODAY }),
      task({ id: '3', dueDate: addDays(TODAY, 1) }),
      task({ id: '4', dueDate: addDays(TODAY, 20) }),
      task({ id: '5', status: 'undated' }),
    ];
    const completed = [task({ id: '9', status: 'completed', completedDate: TODAY })];

    const sections = groupTasks(active, completed, TODAY, DEFAULT_TASKS_SORT);
    expect(sections.map((section) => section.title)).toEqual([
      'Просроченные',
      'Сегодня, 10 сентября',
      'Завтра, 11 сентября',
      '30 сентября',
      'Без даты',
      'Выполненные',
    ]);
  });

  it('датирует секцию чужого года годом', () => {
    const active = [task({ id: '1', dueDate: '2027-05-13' })];
    const sections = groupTasks(active, [], TODAY, DEFAULT_TASKS_SORT);
    expect(sections[0]?.title).toBe('13 мая, 2027');
  });
});

describe('sortTasks — порядок строк внутри групп', () => {
  it('по дате: задачи на весь день раньше задач со временем, время по возрастанию', () => {
    const group = [
      task({ id: '15-40', dueTime: '15:40' }),
      task({ id: 'allday' }),
      task({ id: '09-00', dueTime: '09:00' }),
    ];
    const sorted = sortTasks(group, { field: 'date', direction: 'asc' });
    expect(sorted.map((t) => t.id)).toEqual(['allday', '09-00', '15-40']);
  });

  it('по дате по убыванию: позднее время раньше, весь день — последним', () => {
    const group = [
      task({ id: 'allday' }),
      task({ id: '09-00', dueTime: '09:00' }),
      task({ id: '15-40', dueTime: '15:40' }),
    ];
    const sorted = sortTasks(group, { field: 'date', direction: 'desc' });
    expect(sorted.map((t) => t.id)).toEqual(['15-40', '09-00', 'allday']);
  });

  it('по названию: русская алфавитная сортировка в обе стороны', () => {
    const group = [
      task({ id: 'b', title: 'Поменять замок' }),
      task({ id: 'a', title: 'Вызвать сантехника' }),
      task({ id: 'c', title: 'Позвонить арендатору' }),
    ];
    const asc = sortTasks(group, { field: 'title', direction: 'asc' });
    expect(asc.map((t) => t.id)).toEqual(['a', 'c', 'b']);
    const desc = sortTasks(group, { field: 'title', direction: 'desc' });
    expect(desc.map((t) => t.id)).toEqual(['b', 'c', 'a']);
  });

  it('по дате внутри недатированной группы сортирует по времени создания', () => {
    const group = [
      task({ id: 'old', createdAt: '2026-08-01T10:00:00Z' }),
      task({ id: 'new', createdAt: '2026-09-01T10:00:00Z' }),
    ];
    const sorted = sortTasks(group, { field: 'date', direction: 'asc' });
    expect(sorted.map((t) => t.id)).toEqual(['old', 'new']);
  });
});

describe('parseTasksSortParams — разбор ?sort=&order= экранов задач (#785)', () => {
  it('отсутствие параметров — дефолт «Дата, asc»', () => {
    expect(parseTasksSortParams(undefined, undefined)).toStrictEqual(DEFAULT_TASKS_SORT);
    expect(parseTasksSortParams('', '')).toStrictEqual(DEFAULT_TASKS_SORT);
  });

  it('читает поле и направление; неизвестные значения — дефолтные', () => {
    expect(parseTasksSortParams('title', 'desc')).toStrictEqual({
      field: 'title',
      direction: 'desc',
    });
    expect(parseTasksSortParams('name', 'desc')).toStrictEqual({
      field: 'date',
      direction: 'desc',
    });
    expect(parseTasksSortParams('title', 'up')).toStrictEqual({
      field: 'title',
      direction: 'asc',
    });
  });

  it('массивное значение (битый дубликат параметра) — дефолт, как в книге контактов', () => {
    expect(parseTasksSortParams(['title'], ['desc'])).toStrictEqual(DEFAULT_TASKS_SORT);
  });
});

describe('serializeTasksSortToParams — запись ?sort=&order= (#785)', () => {
  it('дефолт параметров не создаёт', () => {
    expect(serializeTasksSortToParams(DEFAULT_TASKS_SORT)).toStrictEqual({});
  });

  it('пишет только отклонения от дефолта', () => {
    expect(serializeTasksSortToParams({ field: 'title', direction: 'asc' })).toStrictEqual({
      sort: 'title',
    });
    expect(serializeTasksSortToParams({ field: 'date', direction: 'desc' })).toStrictEqual({
      order: 'desc',
    });
    expect(serializeTasksSortToParams({ field: 'title', direction: 'desc' })).toStrictEqual({
      sort: 'title',
      order: 'desc',
    });
  });

  it('обратима: сериализация → разбор возвращает исходную сортировку', () => {
    for (const field of ['date', 'title'] as const) {
      for (const direction of ['asc', 'desc'] as const) {
        const sort: TasksSort = { field, direction };
        const params = serializeTasksSortToParams(sort);
        expect(parseTasksSortParams(params.sort, params.order)).toStrictEqual(sort);
      }
    }
  });
});
