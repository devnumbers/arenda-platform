'use client';

import { useEffect, useState, type JSX, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { Cancel, SmallArrowDown } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { notify } from '@/shared/lib/notifications';
import { formatDayMonth } from '@/entities/task';
import { useProperties, useProperty } from '@/features/properties';
import {
  canCreateTask,
  EMPTY_TASK_CREATE_DRAFT,
  isTaskTitleFilled,
  TASK_REPEAT_OPTIONS,
  useActiveTasks,
  useCreateTaskRule,
  usePropertylessTasks,
  type TaskCreateDraft,
} from '@/features/tasks';
import {
  Button,
  CalendarDatePicker,
  ChipButton,
  IconButton,
  PageContent,
  StickyBottomBar,
  TextField,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import { TaskTimePicker } from './task-time-picker';
import { TaskPropertySelectPage } from './task-property-select';

/** Лимиты контракта POST /tasks/rules (#498): название 1..255, комментарий
 * до 1000. Ввод обрезается молча — счётчиков на макете нет (макет прячет
 * нижние поля Input Field). */
const TITLE_MAX_LENGTH = 255;
const COMMENT_MAX_LENGTH = 1000;

/**
 * Экран «Создать задачу» (#500, Figma 1539-77823/1539-78288/1539-82273):
 * полноэкранная форма поверх списка — страница-маршрут (конвенция
 * полноэкранных поверхностей, фидбек #477) в два шага. Шаг 1 — название
 * (обязательно, «Далее» без него недоступен) и, на входе с глобальной
 * ленты, объект (опционален, #525, макет 1726-88078/88088/87754/89456 —
 * решение владельца 2026-09-07: с объекта поле не показывается, задача
 * всегда на этом объекте); шаг 2 — комментарий, дата/время («Выбрать» →
 * пикеры) и чипы «Повторять каждый» + «Создать» — страница шага
 * необязательна для заполнения, кнопка активна сразу (подсказка
 * владельца). Закрытие — ✕ в шапке (галочки в шапке нет — решение #497);
 * кнопка шага — справа в нижней панели. Дата без срока не нужна — «Без
 * срока» остаётся; время и повтор требуют дату (контракт: дизейбл без
 * даты). «Сегодня» пикеров — серверное today из среза входа (ADR 0048):
 * объектного листинга или, без объекта, глобального (#521). После создания
 * — возврат в список: инвалидация taskKeys перечитывает обе выборки.
 *
 * Объект (#525): только глобальный вход — поле пустое, страница «Выбрать
 * объект» предлагает «Общую задачу» или объект, строгий черновик.
 * Эндпоинт выбирается по объекту черновика: /properties/{id}/tasks/rules
 * или /tasks/rules. Смотрящий и архив глушат мутации (#446, ADR 0028) —
 * на объектном входе форма сразу возвращает на список; глобальное
 * создание всегда в своей книге, гейта нет.
 */
export function TaskCreateScreen({
  initialPropertyId,
}: {
  readonly initialPropertyId: string | null;
}): JSX.Element {
  const router = useRouter();
  const [step, setStep] = useState<1 | 2>(1);
  const [draft, setDraft] = useState<TaskCreateDraft>(() => ({
    ...EMPTY_TASK_CREATE_DRAFT,
    propertyId: initialPropertyId,
  }));
  const [dateOpen, setDateOpen] = useState(false);
  const [timeOpen, setTimeOpen] = useState(false);
  const [objectSelectOpen, setObjectSelectOpen] = useState(false);
  const [propertyDraft, setPropertyDraft] = useState<string | null>(null);

  const propertyQuery = useProperty(initialPropertyId ?? '');
  // «Сегодня» собственника — границы пикера и дефолт выбора; клиент зоны
  // не знает (ADR 0048). Срез — по входу: на объекте листинг объекта,
  // без объекта — глобальный безобъектный (читатель = владелец книги).
  const activeTasksQuery = useActiveTasks(initialPropertyId ?? '');
  const propertylessQuery = usePropertylessTasks({
    enabled: initialPropertyId === null,
  });
  const today =
    initialPropertyId !== null
      ? activeTasksQuery.data?.today
      : propertylessQuery.data?.today;

  const create = useCreateTaskRule();

  const propertiesQuery = useProperties();
  const property = propertyQuery.data;
  const role = property?.access?.role;
  const canMutate =
    property !== undefined && role !== undefined && role !== 'viewer' && property.status !== 'archived';

  useEffect(() => {
    // Гейт только объектного входа: глобальное создание — в своей книге.
    if (initialPropertyId === null) {
      return;
    }
    if (propertyQuery.isSuccess && !canMutate) {
      // Отмена потока, не замена URL: replace оставил бы «мёртвый» Back
      // (стандарт навигации CODING_STANDARDS).
      goBack(router, ROUTES.propertyTasks(initialPropertyId));
    }
  }, [initialPropertyId, propertyQuery.isSuccess, canMutate, router]);

  const close = (): void =>
    goBack(
      router,
      initialPropertyId === null ? ROUTES.tasks : ROUTES.propertyTasks(initialPropertyId),
    );

  const patch = (changes: Partial<TaskCreateDraft>): void =>
    setDraft((prev) => ({ ...prev, ...changes }));

  const submit = (): void => {
    create.mutate(draft, {
      onSuccess: close,
      onError: (error) => notify.scenarios.tasks.createError(error),
    });
  };

  // Поле «Объект» и страница выбора — только глобальный вход (решение
  // владельца 2026-09-07); с объекта задача всегда на этом объекте.
  const objectSelectAvailable = initialPropertyId === null;

  // Подпись поля «Объект»: имя из книги (источник страницы выбора); пока
  // книга не пришла — имя входного объекта из его листинга.
  const selectedProperty = propertiesQuery.data?.find(
    (item) => item.id === draft.propertyId,
  );
  const selectedPropertyName =
    selectedProperty?.name ??
    (draft.propertyId === initialPropertyId ? property?.name : undefined);

  if (objectSelectOpen) {
    return (
      <TaskPropertySelectPage
        draft={propertyDraft}
        onDraftChange={setPropertyDraft}
        onApply={() => {
          patch({ propertyId: propertyDraft });
          setObjectSelectOpen(false);
        }}
        onDismiss={() => setObjectSelectOpen(false)}
      />
    );
  }

  return (
    <>
      <TopNav
        leading={<IconButton icon={<Cancel />} label="Закрыть" onClick={close} />}
      >
        <TopNavTitle title="Создать задачу" />
      </TopNav>

      <PageContent>
        {step === 1 ? (
          <div className={objectSelectAvailable ? 'flex flex-col gap-8 px-6' : 'px-6'}>
            <TextField
              title="Задача"
              value={draft.title}
              onChange={(event) =>
                patch({ title: event.target.value.slice(0, TITLE_MAX_LENGTH) })
              }
              onClear={() => patch({ title: '' })}
            />
            {objectSelectAvailable && (
              <TaskFieldButton
                title="Объект"
                label={selectedPropertyName ?? 'Выбрать объект'}
                description="Необязательно"
                trailingIcon={<SmallArrowDown />}
                onClick={() => {
                  setPropertyDraft(draft.propertyId);
                  setObjectSelectOpen(true);
                }}
              />
            )}
          </div>
        ) : (
          <div className="flex flex-col gap-8 px-6">
            <TextField
              multiline
              autoGrow
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
        <CalendarDatePicker
          today={today}
          value={draft.dueDate}
          onClose={() => setDateOpen(false)}
          onConfirm={(dueDate) => {
            if (dueDate === null) {
              // Дата снята — время и повтор без неё невалидны, чистый сброс
              // (решение владельца 2026-09-03): «Создать» остаётся активной.
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

/** Поле-кнопка «Дата»/«Время» (макет 1539-82273): бокс TextField (h-14,
 * radius 16, серый) с меткой Title Out, открывает пикер. Общая с экраном
 * правки (#502). У поля «Объект» (#525, макет 1726-88078) — описание под
 * боксом и декоративная стрелка справа (стили нижней строки и «хвоста» —
 * как у TextField). */
export function TaskFieldButton({
  title,
  label,
  description,
  trailingIcon,
  disabled = false,
  onClick,
}: {
  readonly title: string;
  readonly label: string;
  readonly description?: string;
  readonly trailingIcon?: ReactNode;
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
        <span className="min-w-0 flex-1 truncate">{label}</span>
        {trailingIcon !== undefined && (
          <span aria-hidden className="pl-1">
            {trailingIcon}
          </span>
        )}
      </button>
      {description !== undefined && (
        <span className="text-[13px] leading-[15px] text-content-tertiary">{description}</span>
      )}
    </div>
  );
}
