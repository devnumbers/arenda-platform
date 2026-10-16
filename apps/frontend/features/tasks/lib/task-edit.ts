/**
 * Домен-логика формы «Изменить задачу» (#502, Figma 1549-91324): экран
 * правит правило (словарь #494) — будущие вхождения перематериализуются
 * свежими снимками, выполненные остаются со своими. Контракт
 * PATCH /tasks/rules/{ruleId} (#498): частичное обновление — в запрос
 * уходят только изменённые поля; comment, dueDate и dueTime — три-стейт,
 * null очищает. Название обязательно (1..255), время и повтор требуют
 * дату — как при создании; «без повтора» — once (чип снят).
 */

import type { components } from '@/shared/api/dto';
import type { IsoDate, TaskRule } from '@/entities/task';
import {
  canCreateTask,
  type TaskRepeatChoice,
} from './task-create';

/** Команда PATCH /properties/{id}/tasks/rules/{ruleId}: дифф черновика
 * против правила — только изменённые поля. */
export type TaskRuleUpdateCommand = components['schemas']['TaskRuleUpdateRequest'];

/** Черновик формы правки: структура та же, что у создания (#500). */
export type TaskEditDraft = {
  readonly title: string;
  readonly comment: string;
  readonly dueDate: IsoDate | null;
  /** HH:MM из пикера времени; null — на весь день. */
  readonly dueTime: string | null;
  readonly repeat: TaskRepeatChoice | null;
};

/** Черновик из правила: comment null → пустая строка поля, once → чип снят. */
export function initialTaskEditDraft(rule: TaskRule): TaskEditDraft {
  return {
    title: rule.title,
    comment: rule.comment ?? '',
    dueDate: rule.dueDate,
    dueTime: rule.dueTime,
    repeat: rule.repeat === 'once' ? null : rule.repeat,
  };
}

/**
 * Валидность черновика правки совпадает с созданием: контракты POST и
 * PATCH (#498) накладывают одни и те же зависимости — название обязательно,
 * время и повтор требуют дату.
 */
export const canSaveTask = canCreateTask;

/** Payload PATCH /properties/{id}/tasks/rules/{ruleId}; null — черновик
 * невалиден либо изменений нет (кнопка «Сохранить» задизейблена). */
export function buildTaskRuleUpdateRequest(
  rule: TaskRule,
  draft: TaskEditDraft,
): TaskRuleUpdateCommand | null {
  if (!canSaveTask(draft)) {
    return null;
  }
  const request: TaskRuleUpdateCommand = {};
  const title = draft.title.trim();
  if (title !== rule.title) {
    request.title = title;
  }
  const comment = draft.comment.trim();
  if (comment !== (rule.comment ?? '')) {
    // Три-стейт: пустой комментарий очищает (null), непустой — значение.
    request.comment = comment.length > 0 ? comment : null;
  }
  if (draft.dueDate !== rule.dueDate) {
    request.dueDate = draft.dueDate;
  }
  if (draft.dueTime !== rule.dueTime) {
    request.dueTime = draft.dueTime;
  }
  const repeat = draft.repeat ?? 'once';
  if (repeat !== rule.repeat) {
    request.repeat = repeat;
  }
  return Object.keys(request).length > 0 ? request : null;
}
