import { describe, expect, it } from 'vitest';
import {
  buildTaskRuleCreateRequest,
  canCreateTask,
  EMPTY_TASK_CREATE_DRAFT,
  isTaskTitleFilled,
  TASK_REPEAT_OPTIONS,
  taskRuleCreatePath,
  type TaskCreateDraft,
} from './task-create';

function draft(partial: Partial<TaskCreateDraft>): TaskCreateDraft {
  return { ...EMPTY_TASK_CREATE_DRAFT, ...partial };
}

describe('TASK_REPEAT_OPTIONS — чипы «Повторять каждый»', () => {
  it('перечисляет День/Неделю/Месяц/Год значениями контракта', () => {
    expect(TASK_REPEAT_OPTIONS).toEqual([
      { value: 'daily', label: 'День' },
      { value: 'weekly', label: 'Неделю' },
      { value: 'monthly', label: 'Месяц' },
      { value: 'yearly', label: 'Год' },
    ]);
  });
});

describe('isTaskTitleFilled — шаг 1, «Далее» без названия недоступен', () => {
  it('название непустое — заполнено', () => {
    expect(isTaskTitleFilled('Вызвать сантехника')).toBe(true);
  });

  it('пустое и пробельное название — не заполнено', () => {
    expect(isTaskTitleFilled('')).toBe(false);
    expect(isTaskTitleFilled('   ')).toBe(false);
  });
});

describe('canCreateTask — контрактные зависимости шага 2', () => {
  it('без названия создать нельзя', () => {
    expect(canCreateTask(draft({ title: '  ' }))).toBe(false);
  });

  it('только название — достаточно (всё остальное необязательно)', () => {
    expect(canCreateTask(draft({ title: 'Полить цветы' }))).toBe(true);
  });

  it('время без даты — нельзя (время требует дату)', () => {
    expect(
      canCreateTask(draft({ title: 'Т', dueTime: '12:00' })),
    ).toBe(false);
    expect(
      canCreateTask(draft({ title: 'Т', dueDate: '2026-09-10', dueTime: '12:00' })),
    ).toBe(true);
  });

  it('повтор без даты — нельзя (повтор требует дату)', () => {
    expect(canCreateTask(draft({ title: 'Т', repeat: 'weekly' }))).toBe(false);
    expect(
      canCreateTask(draft({ title: 'Т', dueDate: '2026-09-10', repeat: 'weekly' })),
    ).toBe(true);
  });
});

describe('buildTaskRuleCreateRequest — payload POST /tasks/rules', () => {
  it('только название: repeat = once, остальное опущено', () => {
    expect(buildTaskRuleCreateRequest(draft({ title: '  Разобрать кладовку ' }))).toEqual({
      title: 'Разобрать кладовку',
      repeat: 'once',
    });
  });

  it('полный черновик передаёт все поля', () => {
    expect(
      buildTaskRuleCreateRequest(
        draft({
          title: 'Полить цветы',
          comment: '  На балконе и в кухне  ',
          dueDate: '2026-09-10',
          dueTime: '21:00',
          repeat: 'weekly',
        }),
      ),
    ).toEqual({
      title: 'Полить цветы',
      comment: 'На балконе и в кухне',
      dueDate: '2026-09-10',
      dueTime: '21:00',
      repeat: 'weekly',
    });
  });

  it('пробельный комментарий опускается', () => {
    const request = buildTaskRuleCreateRequest(draft({ title: 'Т', comment: '   ' }));
    expect(request).not.toBeNull();
    expect(request?.comment).toBeUndefined();
  });

  it('нарушенный контракт (время без даты) не превращается в запрос', () => {
    expect(buildTaskRuleCreateRequest(draft({ title: 'Т', dueTime: '09:00' }))).toBeNull();
  });
});

describe('черновик и объект (#525)', () => {
  it('пустой черновик — без объекта', () => {
    expect(EMPTY_TASK_CREATE_DRAFT.propertyId).toBeNull();
  });

  it('объект не влияет на валидацию: создание без объекта и с объектом равнозначно', () => {
    expect(canCreateTask(draft({ title: 'Т' }))).toBe(true);
    expect(canCreateTask(draft({ title: 'Т', propertyId: 'p1' }))).toBe(true);
  });
});

describe('taskRuleCreatePath — выбор эндпоинта по объекту (#525)', () => {
  it('без объекта — глобальный POST /tasks/rules (#520)', () => {
    expect(taskRuleCreatePath(null)).toBe('/tasks/rules');
  });

  it('с объектом — объектный POST /properties/{id}/tasks/rules', () => {
    expect(taskRuleCreatePath('p1')).toBe('/properties/p1/tasks/rules');
    expect(taskRuleCreatePath('a/b')).toBe('/properties/a%2Fb/tasks/rules');
  });
});

