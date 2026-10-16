import { describe, expect, it } from 'vitest';
import type { TaskRule } from '@/entities/task';
import {
  buildTaskRuleUpdateRequest,
  canSaveTask,
  initialTaskEditDraft,
  type TaskEditDraft,
} from './task-edit';

const RULE: TaskRule = {
  id: 'rule-1',
  propertyId: 'property-1',
  title: 'Полить цветы',
  comment: null,
  dueDate: '2026-09-10',
  dueTime: null,
  repeat: 'once',
  createdAt: '2026-09-01T10:00:00Z',
  updatedAt: '2026-09-01T10:00:00Z',
};

function rule(partial: Partial<TaskRule>): TaskRule {
  return { ...RULE, ...partial };
}

function draft(partial: Partial<TaskEditDraft>): TaskEditDraft {
  return { ...initialTaskEditDraft(RULE), ...partial };
}

describe('initialTaskEditDraft — черновик из правила', () => {
  it('переносит поля правила: comment null → пустая строка, once → чип снят', () => {
    expect(initialTaskEditDraft(RULE)).toStrictEqual({
      title: 'Полить цветы',
      comment: '',
      dueDate: '2026-09-10',
      dueTime: null,
      repeat: null,
    });
  });

  it('непустой комментарий и повтор без once переносятся как есть', () => {
    expect(
      initialTaskEditDraft(
        rule({ comment: 'Позвонить арендатору', repeat: 'weekly', dueTime: '12:00' }),
      ),
    ).toStrictEqual({
      title: 'Полить цветы',
      comment: 'Позвонить арендатору',
      dueDate: '2026-09-10',
      dueTime: '12:00',
      repeat: 'weekly',
    });
  });
});

describe('canSaveTask — валидность черновика правки', () => {
  it('пустое название — сохранить нельзя', () => {
    expect(canSaveTask(draft({ title: '  ' }))).toBe(false);
  });

  it('заполненное название — достаточно', () => {
    expect(canSaveTask(draft({}))).toBe(true);
  });

  it('время без даты — нельзя (время требует дату)', () => {
    expect(canSaveTask(draft({ dueDate: null, dueTime: '12:00' }))).toBe(false);
  });

  it('повтор без даты — нельзя (повтор требует дату)', () => {
    expect(canSaveTask(draft({ dueDate: null, repeat: 'weekly' }))).toBe(false);
  });
});

describe('buildTaskRuleUpdateRequest — дифф черновика против правила', () => {
  it('изменений нет — null (кнопка «Сохранить» задизейблена)', () => {
    expect(buildTaskRuleUpdateRequest(RULE, draft({}))).toBeNull();
  });

  it('черновик невалиден — null даже при изменениях', () => {
    expect(buildTaskRuleUpdateRequest(RULE, draft({ title: '  ' }))).toBeNull();
  });

  it('название триммится; пробелы без смены текста — не изменение', () => {
    expect(buildTaskRuleUpdateRequest(RULE, draft({ title: '  Полить розы  ' }))).toStrictEqual({
      title: 'Полить розы',
    });
    expect(buildTaskRuleUpdateRequest(RULE, draft({ title: ' Полить цветы ' }))).toBeNull();
  });

  it('комментарий установлен — строка в запросе', () => {
    expect(buildTaskRuleUpdateRequest(RULE, draft({ comment: 'Тёплой водой' }))).toStrictEqual({
      comment: 'Тёплой водой',
    });
  });

  it('комментарий очищен — три-стейт null; без изменений — поле не уходит', () => {
    const withComment = rule({ comment: 'Тёплой водой' });
    expect(
      buildTaskRuleUpdateRequest(withComment, {
        ...initialTaskEditDraft(withComment),
        comment: '   ',
      }),
    ).toStrictEqual({ comment: null });
    expect(
      buildTaskRuleUpdateRequest(withComment, initialTaskEditDraft(withComment)),
    ).toBeNull();
  });

  it('дата изменена — новое значение; дата снята — три-стейт null', () => {
    expect(buildTaskRuleUpdateRequest(RULE, draft({ dueDate: '2026-10-01' }))).toStrictEqual({
      dueDate: '2026-10-01',
    });
    expect(buildTaskRuleUpdateRequest(RULE, draft({ dueDate: null }))).toStrictEqual({
      dueDate: null,
    });
  });

  it('время установлено и снято — значение и три-стейт null', () => {
    expect(buildTaskRuleUpdateRequest(RULE, draft({ dueTime: '12:00' }))).toStrictEqual({
      dueTime: '12:00',
    });
    const withTime = rule({ dueTime: '12:00' });
    expect(
      buildTaskRuleUpdateRequest(withTime, {
        ...initialTaskEditDraft(withTime),
        dueTime: null,
      }),
    ).toStrictEqual({ dueTime: null });
  });

  it('повтор: once → weekly уходит weekly, weekly → чип снят уходит once', () => {
    expect(buildTaskRuleUpdateRequest(RULE, draft({ repeat: 'weekly' }))).toStrictEqual({
      repeat: 'weekly',
    });
    expect(
      buildTaskRuleUpdateRequest(rule({ repeat: 'weekly' }), draft({ repeat: null })),
    ).toStrictEqual({ repeat: 'once' });
  });

  it('смена нескольких полей уходит одним запросом', () => {
    expect(
      buildTaskRuleUpdateRequest(
        RULE,
        draft({ title: 'Полить розы', dueDate: '2026-10-01', dueTime: '09:00', repeat: 'weekly' }),
      ),
    ).toStrictEqual({
      title: 'Полить розы',
      dueDate: '2026-10-01',
      dueTime: '09:00',
      repeat: 'weekly',
    });
  });
});
