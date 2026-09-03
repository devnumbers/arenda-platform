'use client';

import { useEffect, useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { Cancel } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { notify } from '@/shared/lib/notifications';
import { formatDayMonth, type IsoDate, type TaskRule } from '@/entities/task';
import { useProperty } from '@/features/properties';
import {
  buildTaskRuleUpdateRequest,
  initialTaskEditDraft,
  TASK_REPEAT_OPTIONS,
  useActiveTasks,
  useTaskRule,
  useUpdateTaskRule,
  type TaskEditDraft,
} from '@/features/tasks';
import {
  Button,
  ChipButton,
  IconButton,
  PageContent,
  StickyBottomBar,
  TextField,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import { TaskFieldButton } from './task-create-screen';
import { TaskDatePicker } from './task-date-picker';
import { TaskTimePicker } from './task-time-picker';
import { TasksSkeleton, TasksStateCard } from './tasks-of-property-screen';

/** Лимиты контракта PATCH /tasks/rules/{ruleId} (#498): название 1..255,
 * комментарий до 1000. Ввод обрезается молча — счётчиков на макете нет. */
const TITLE_MAX_LENGTH = 255;
const COMMENT_MAX_LENGTH = 1000;

/**
 * Экран «Изменить задачу» (#502, Figma 1549-91324): одноэкранная форма
 * правки правила (словарь #494) — та же анатомия, что шаг 2 создания
 * (#500), плюс поле названия с очисткой ✕; без шагов и галочки в шапке
 * (решение #497). Семантика: правится правило — будущие вхождения
 * перематериализуются свежими снимками, выполненные остаются со своими
 * (сообщение об этом — success-тост после сохранения, формат решается на
 * приёмке). Сохранение — частичный PATCH: запрос — дифф черновика против
 * правила (lib/task-edit), без изменений «Сохранить» недоступна; снятие
 * даты повторным тапом чистит время и повтор (решение владельца к #500).
 * «Сегодня» пикеров — серверное today собственника из кэша листинга
 * (ADR 0048). После сохранения — возврат в список. Смотрящий и архив
 * глушат мутации (#446, ADR 0028) — форма сразу возвращает на список.
 */
export function TaskEditScreen({
  propertyId,
  ruleId,
}: {
  readonly propertyId: string;
  readonly ruleId: string;
}): JSX.Element {
  const router = useRouter();
  const propertyQuery = useProperty(propertyId);
  const ruleQuery = useTaskRule(propertyId, ruleId);
  // «Сегодня» собственника — границы пикера (ADR 0048); источник — кэш
  // листинга, как на создании.
  const activeTasksQuery = useActiveTasks(propertyId);
  const today = activeTasksQuery.data?.today;

  const property = propertyQuery.isSuccess ? propertyQuery.data : undefined;
  const role = property?.access?.role;
  const canMutate =
    property !== undefined && role !== undefined && role !== 'viewer' && property.status !== 'archived';

  useEffect(() => {
    if (propertyQuery.isSuccess && !canMutate) {
      // Отмена потока, не замена URL: replace оставил бы «мёртвый» Back
      // (стандарт навигации CODING_STANDARDS).
      goBack(router, ROUTES.propertyTasks(propertyId));
    }
  }, [propertyQuery.isSuccess, canMutate, router, propertyId]);

  const close = (): void => goBack(router, ROUTES.propertyTasks(propertyId));

  const rule = ruleQuery.isSuccess ? ruleQuery.data : undefined;
  const loading = propertyQuery.isPending || ruleQuery.isPending;
  const failed = propertyQuery.isError || ruleQuery.isError;

  return (
    <>
      <TopNav
        leading={<IconButton icon={<Cancel />} label="Закрыть" onClick={close} />}
      >
        <TopNavTitle title="Изменить задачу" />
      </TopNav>

      <PageContent>
        <div className="flex flex-col gap-8">
          {loading && (
            <>
              <TasksSkeleton />
              <TasksSkeleton />
            </>
          )}

          {!loading && failed && (
            <TasksStateCard
              title="Не удалось загрузить задачу"
              onRetry={() => {
                void ruleQuery.refetch();
                void propertyQuery.refetch();
              }}
            />
          )}

          {!loading && !failed && rule !== undefined && canMutate && (
            // key — на случай переиспользования смонтированной формы под
            // другое правило (черновик не должен пережить смену источника).
            <TaskEditForm key={rule.id} rule={rule} today={today} />
          )}
        </div>
      </PageContent>
    </>
  );
}

/** Форма правки: черновик живёт в состоянии; в PATCH уходит только
 * команда-дифф против правила (как у правки платежа — команду собирает
 * lib/task-edit). */
function TaskEditForm({
  rule,
  today,
}: {
  readonly rule: TaskRule;
  readonly today: IsoDate | undefined;
}): JSX.Element {
  const router = useRouter();
  const [draft, setDraft] = useState<TaskEditDraft>(() => initialTaskEditDraft(rule));
  const [dateOpen, setDateOpen] = useState(false);
  const [timeOpen, setTimeOpen] = useState(false);

  const update = useUpdateTaskRule(rule.propertyId, rule.id);

  const close = (): void => goBack(router, ROUTES.propertyTasks(rule.propertyId));

  const patch = (changes: Partial<TaskEditDraft>): void =>
    setDraft((prev) => ({ ...prev, ...changes }));

  // Дифф против правила: null — черновик невалиден либо изменений нет
  // («Сохранить» задизейблена).
  const command = buildTaskRuleUpdateRequest(rule, draft);
  const canSave = command !== null;

  const submit = (): void => {
    if (command === null) {
      return;
    }
    update.mutate(command, {
      onSuccess: () => {
        // Тост живёт в глобальном портале — виден уже на списке.
        notify.scenarios.tasks.updated();
        close();
      },
      onError: (error) => notify.scenarios.tasks.updateError(error),
    });
  };

  return (
    <>
      <div className="flex flex-col gap-8 px-6">
        <TextField
          title="Задача"
          value={draft.title}
          onChange={(event) =>
            patch({ title: event.target.value.slice(0, TITLE_MAX_LENGTH) })
          }
          onClear={() => patch({ title: '' })}
        />
        <TextField
          multiline
          title="Комментарий"
          value={draft.comment}
          onChange={(event) =>
            patch({ comment: event.target.value.slice(0, COMMENT_MAX_LENGTH) })
          }
        />
        <div className="grid grid-cols-2 gap-2">
          {/* «Сегодня» ещё не пришло из листинга — пикеру не на что
              опереться (ADR 0048), кнопка ждёт. */}
          <TaskFieldButton
            title="Дата"
            label={draft.dueDate === null ? 'Выбрать' : formatDayMonth(draft.dueDate)}
            disabled={today === undefined}
            onClick={() => setDateOpen(true)}
          />
          {/* Время требует дату (контракт) — без неё пикер недоступен. */}
          <TaskFieldButton
            title="Время"
            label={draft.dueTime ?? 'Выбрать'}
            disabled={draft.dueDate === null}
            onClick={() => setTimeOpen(true)}
          />
        </div>
        <div className="flex flex-col gap-3">
          <span className="text-base font-medium leading-[18px] text-content">
            Повторять каждый
          </span>
          <div className="flex flex-wrap gap-1">
            {TASK_REPEAT_OPTIONS.map((option) => (
              <ChipButton
                key={option.value}
                selected={draft.repeat === option.value}
                disabled={draft.dueDate === null}
                onClick={() =>
                  patch({
                    repeat: draft.repeat === option.value ? null : option.value,
                  })
                }
              >
                {option.label}
              </ChipButton>
            ))}
          </div>
        </div>
      </div>

      <StickyBottomBar>
        {/* Кнопка справа, как на создании (Figma 1549-91327). */}
        <div className="flex justify-end">
          <Button loading={update.isPending} disabled={!canSave} onClick={submit}>
            Сохранить
          </Button>
        </div>
      </StickyBottomBar>

      {dateOpen && today !== undefined && (
        <TaskDatePicker
          today={today}
          value={draft.dueDate}
          onClose={() => setDateOpen(false)}
          onConfirm={(dueDate) => {
            if (dueDate === null) {
              // Дата снята — время и повтор без неё невалидны, чистый сброс
              // (решение владельца 2026-09-03): «Сохранить» не блокируется.
              patch({ dueDate: null, dueTime: null, repeat: null });
            } else {
              patch({ dueDate });
            }
            setDateOpen(false);
          }}
        />
      )}

      <TaskTimePicker
        open={timeOpen}
        onOpenChange={setTimeOpen}
        value={draft.dueTime}
        onConfirm={(dueTime) => {
          patch({ dueTime });
          setTimeOpen(false);
        }}
      />
    </>
  );
}
