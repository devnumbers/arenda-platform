'use client';

import { useEffect, useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { Cancel } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { notify } from '@/shared/lib/notifications';
import { formatDayMonth } from '@/entities/task';
import { useProperty } from '@/features/properties';
import {
  canCreateTask,
  EMPTY_TASK_CREATE_DRAFT,
  isTaskTitleFilled,
  TASK_REPEAT_OPTIONS,
  useActiveTasks,
  useCreateTaskRule,
  type TaskCreateDraft,
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
import { TaskDatePicker } from './task-date-picker';
import { TaskTimePicker } from './task-time-picker';

/** Лимиты контракта POST /tasks/rules (#498): название 1..255, комментарий
 * до 1000. Ввод обрезается молча — счётчиков на макете нет (макет прячет
 * нижние поля Input Field). */
const TITLE_MAX_LENGTH = 255;
const COMMENT_MAX_LENGTH = 1000;

/**
 * Экран «Создать задачу» (#500, Figma 1539-77823/1539-78288/1539-82273):
 * полноэкранная форма поверх списка — страница-маршрут (конвенция
 * полноэкранных поверхностей, фидбек #477) в два шага. Шаг 1 — название
 * (обязательно, «Далее» без него недоступен); шаг 2 — комментарий,
 * дата/время («Выбрать» → пикеры) и чипы «Повторять каждый» + «Создать» —
 * страница шага необязательна для заполнения, кнопка активна сразу
 * (подсказка владельца). Закрытие — ✕ в шапке (галочки в шапке нет —
 * решение #497); кнопка шага — справа в нижней панели. Дата без срока не
 * нужна — «Без срока» остаётся; время и повтор требуют дату (контракт:
 * дизейбл без даты). «Сегодня» пикеров — серверное today собственника из
 * кэша листинга (ADR 0048). После создания — возврат в список: инвалидация
 * taskKeys перечитывает обе выборки. Смотрящий и архив глушат мутации
 * (#446, ADR 0028) — форма сразу возвращает на список.
 */
export function TaskCreateScreen({
  propertyId,
}: {
  readonly propertyId: string;
}): JSX.Element {
  const router = useRouter();
  const [step, setStep] = useState<1 | 2>(1);
  const [draft, setDraft] = useState<TaskCreateDraft>(EMPTY_TASK_CREATE_DRAFT);
  const [dateOpen, setDateOpen] = useState(false);
  const [timeOpen, setTimeOpen] = useState(false);

  const propertyQuery = useProperty(propertyId);
  // «Сегодня» собственника — границы пикера и дефолт выбора; клиент зоны
  // не знает (ADR 0048), источника кроме листинга у формы нет.
  const activeTasksQuery = useActiveTasks(propertyId);
  const today = activeTasksQuery.data?.today;

  const create = useCreateTaskRule(propertyId);

  const property = propertyQuery.data;
  const role = property?.access?.role;
  const canMutate =
    property !== undefined && role !== undefined && role !== 'viewer' && property.status !== 'archived';

  useEffect(() => {
    if (propertyQuery.isSuccess && !canMutate) {
      router.replace(ROUTES.propertyTasks(propertyId));
    }
  }, [propertyQuery.isSuccess, canMutate, router, propertyId]);

  const close = (): void => goBack(router, ROUTES.propertyTasks(propertyId));

  const patch = (changes: Partial<TaskCreateDraft>): void =>
    setDraft((prev) => ({ ...prev, ...changes }));

  const submit = (): void => {
    create.mutate(draft, {
      onSuccess: close,
      onError: (error) => notify.scenarios.tasks.createError(error),
    });
  };

  return (
    <>
      <TopNav
        leading={<IconButton icon={<Cancel />} label="Закрыть" onClick={close} />}
      >
        <TopNavTitle title="Создать задачу" />
      </TopNav>

      <PageContent>
        {step === 1 ? (
          <div className="px-6">
            <TextField
              title="Задача"
              value={draft.title}
              onChange={(event) =>
                patch({ title: event.target.value.slice(0, TITLE_MAX_LENGTH) })
              }
              onClear={() => patch({ title: '' })}
            />
          </div>
        ) : (
          <div className="flex flex-col gap-8 px-6">
            <TextField
              multiline
              title="Комментарий"
              value={draft.comment}
              onChange={(event) =>
                patch({ comment: event.target.value.slice(0, COMMENT_MAX_LENGTH) })
              }
            />
            <div className="grid grid-cols-2 gap-2">
              <TaskFieldButton
                title="Дата"
                label={draft.dueDate === null ? 'Выбрать' : formatDayMonth(draft.dueDate)}
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
        )}
      </PageContent>

      <StickyBottomBar>
        <div className="flex justify-end">
          {step === 1 ? (
            <Button disabled={!isTaskTitleFilled(draft.title)} onClick={() => setStep(2)}>
              Далее
            </Button>
          ) : (
            <Button loading={create.isPending} disabled={!canCreateTask(draft)} onClick={submit}>
              Создать
            </Button>
          )}
        </div>
      </StickyBottomBar>

      {dateOpen && today !== undefined && (
        <TaskDatePicker
          today={today}
          value={draft.dueDate}
          onClose={() => setDateOpen(false)}
          onConfirm={(dueDate) => {
            patch({ dueDate });
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

/** Поле-кнопка «Дата»/«Время» (макет 1539-82273): бокс TextField (h-14,
 * radius 16, серый) с меткой Title Out, открывает пикер. */
function TaskFieldButton({
  title,
  label,
  disabled = false,
  onClick,
}: {
  readonly title: string;
  readonly label: string;
  readonly disabled?: boolean;
  readonly onClick: () => void;
}): JSX.Element {
  return (
    <div className={`flex flex-col gap-2${disabled ? ' opacity-50' : ''}`}>
      <span className="text-base font-medium leading-[18px] text-content">{title}</span>
      <button
        type="button"
        disabled={disabled}
        onClick={onClick}
        className="flex h-14 w-full cursor-pointer items-center rounded-button bg-surface-muted pl-[18px] pr-2 text-left text-base leading-[18px] text-content outline-none transition-shadow hover:shadow-[inset_0_0_0_2px_var(--dl-input-border)] focus-visible:ring-2 focus-visible:ring-primary disabled:pointer-events-none"
      >
        {label}
      </button>
    </div>
  );
}
