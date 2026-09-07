/**
 * Домен-логика формы «Создать задачу» (#500, Figma 1539-77823/1539-78288/
 * 1539-82273): два шага — название, затем комментарий/дата/время/повтор.
 * Контракт POST /tasks/rules (#498): название обязательно (1..255), дата
 * опциональна — правило без даты даёт задачу «Без срока»; время и повтор
 * требуют дату; repeat в запросе обязателен — «без повтора» это once
 * (тап по выбранному чипу снимает его). Чипы «Повторять каждый» —
 * аккузатив «повторять каждый день/неделю/месяц/год» (решение #497).
 */

import type { components } from '@/shared/api/dto';
import type { IsoDate } from '@/entities/task';

type TaskRuleCreateRequestDto = components['schemas']['TaskRuleCreateRequest'];

/** Выбор чипа повтора: значение контракта без once (once = чип снят). */
export type TaskRepeatChoice = Exclude<components['schemas']['TaskRepeat'], 'once'>;

/** Опции чипов «Повторять каждый» в порядке макета (1539-82273). */
export const TASK_REPEAT_OPTIONS: ReadonlyArray<{
  readonly value: TaskRepeatChoice;
  readonly label: string;
}> = [
  { value: 'daily', label: 'День' },
  { value: 'weekly', label: 'Неделю' },
  { value: 'monthly', label: 'Месяц' },
  { value: 'yearly', label: 'Год' },
];

/** Черновик формы: всё, кроме названия, необязательно. Объект тоже: null —
 * «Общая задача», правило без объекта (#525). */
export type TaskCreateDraft = {
  readonly title: string;
  readonly comment: string;
  /** Привязка правила: uuid объекта или null — правило без объекта (ADR 0052). */
  readonly propertyId: string | null;
  readonly dueDate: IsoDate | null;
  /** HH:MM из пикера времени; null — на весь день. */
  readonly dueTime: string | null;
  readonly repeat: TaskRepeatChoice | null;
};

export const EMPTY_TASK_CREATE_DRAFT: TaskCreateDraft = {
  title: '',
  comment: '',
  propertyId: null,
  dueDate: null,
  dueTime: null,
  repeat: null,
};

/** Название обязательно — «Далее» шага 1 и «Создать» без него недоступны. */
export function isTaskTitleFilled(title: string): boolean {
  return title.trim().length > 0;
}

/**
 * Контрактные зависимости шага 2: время и повтор требуют дату. Экран
 * дизейблит чипы и пикер времени без даты — здесь последняя линия
 * перед запросом. Объект не участвует — общая с правкой (#502): принимает
 * и её черновик.
 */
export function canCreateTask(
  draft: Pick<TaskCreateDraft, 'title' | 'dueDate' | 'dueTime' | 'repeat'>,
): boolean {
  if (!isTaskTitleFilled(draft.title)) {
    return false;
  }
  if (draft.dueTime !== null && draft.dueDate === null) {
    return false;
  }
  if (draft.repeat !== null && draft.dueDate === null) {
    return false;
  }
  return true;
}

/** Payload POST создания правила; null — черновик невалиден. Объект в теле
 * не участвует — им выбирается эндпоинт (taskRuleCreatePath). */
export function buildTaskRuleCreateRequest(
  draft: TaskCreateDraft,
): TaskRuleCreateRequestDto | null {
  if (!canCreateTask(draft)) {
    return null;
  }
  const comment = draft.comment.trim();
  return {
    title: draft.title.trim(),
    ...(comment.length > 0 ? { comment } : {}),
    ...(draft.dueDate !== null ? { dueDate: draft.dueDate } : {}),
    ...(draft.dueTime !== null ? { dueTime: draft.dueTime } : {}),
    repeat: draft.repeat ?? 'once',
  };
}

/**
 * Эндпоинт создания по объекту черновика (#525): с объектом — объектный
 * POST /properties/{id}/tasks/rules, без («Общая задача») — глобальный
 * POST /tasks/rules (#520). Общего «создать с объектом в теле» в контракте
 * нет — срез правила задаёт путь.
 */
export function taskRuleCreatePath(propertyId: string | null): string {
  return propertyId === null
    ? '/tasks/rules'
    : `/properties/${encodeURIComponent(propertyId)}/tasks/rules`;
}
