'use client';

import { useEffect, useMemo, useState } from 'react';
import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import {
  Add,
  Checkmark,
  SmallArrowDown,
  TrashBin,
  VerticalMenu,
} from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { ApiError } from '@/shared/api/errors';
import { notify } from '@/shared/lib/notifications';
import { useProperties } from '@/features/properties';
import {
  DEFAULT_TASKS_SORT,
  canMutateFeedTask,
  groupTasks,
  mutableFeedTasks,
  useCompleteAllGlobalTasks,
  useCompleteGlobalTask,
  useDeleteCompletedGlobalTasks,
  useGlobalActiveTasks,
  useGlobalCompletedTasks,
  useTasksFeedFilter,
  type FeedPropertyRef,
  type TaskSection,
  type TasksSort,
} from '@/features/tasks';
import type { Task } from '@/entities/task';
import {
  ChipButton,
  CollapsibleSection,
  EmptyState,
  IconButton,
  Menu,
  MenuContent,
  MenuItem,
  MenuTrigger,
  PageContent,
  PickerMenu,
  TopNav,
} from '@/shared/ui/design';
import { TaskRow } from './task-row';
import { TaskSectionCard } from './task-section-card';
import { TasksDeleteCompletedDialog } from './tasks-delete-completed-dialog';
import { TasksPropertySelectPage } from './tasks-property-select';
import { TasksSkeleton, TasksStateCard } from './tasks-of-property-screen';
import { sectionKey, sectionTone } from './tasks-section-utils';
import { SortChip, sortPickerGroups } from './tasks-sort';

/**
 * Экран «Задачи» — глобальная лента (карта #518, тикет #523, Figma
 * 1733-27411/1726-86913): merged-фид читателя из GET /tasks (#521) — свои
 * задачи и задачи видимых объектов, архивы мимо (решение 9 #522). Секции —
 * канон объектного экрана (#499): границы «Сегодня»/«Завтра» считает сервер
 * по календаре читателя, порядок секций приходит плоским по сроку, строки
 * внутри сортируются клиентски (дефолт «Дата, asc» — решение 5 #522; чип
 * «Название» на макете — не дефолт). У объектных строк — строка объекта
 * (HomeMainSmall + имя, propertyName из контракта #521); тап по активной
 * объектной строке — правка на объекте, по безобъектной — плоский маршрут
 * /tasks/{ruleId}/edit (#537, решения 2–3 #522), выполненные некликабельны.
 * Кебаб — «Отметить все» (только мутабельные строки) и «Удалить выполненные»
 * (глобальный DELETE /tasks/completed, #536); мутации выполнения
 * маршрутизируются по срезу (ADR 0052), зритель читает (ADR 0028).
 *
 * Шапка — стандартный TopNav хаба с «крыльями» и на мобайле (`mobileWings`,
 * Figma 1733-27411); смена на компактный заголовок по прокрутке (1733-92349)
 * отложена — решение владельца 2026-09-05. Чип «Объект» — фильтр по
 * объектам (#524/#547, Figma 1726-88880/1726-86913): выбранные объекты
 * живут в адресе (?property=<id>[,<id>…], мультивыбор — решение владельца
 * 2026-09-05), страница выбора — строгий черновик без истории, при 404
 * фильтр сбрасывается сам (решение 8 #522). Подпись чипа всегда «Объект»,
 * активное состояние — признак включённого фильтра (решение владельца
 * 2026-09-05, перекрывает «чип = имя объекта» из решения 7 #522).
 * «+» нарисовано по макету, но пока без действия — создание, тикет #525.
 */
export function TasksFeedScreen(): JSX.Element {
  const router = useRouter();

  const { filter, applyPropertyFilter } = useTasksFeedFilter();
  const activeQuery = useGlobalActiveTasks(filter.propertyIds);
  const completedQuery = useGlobalCompletedTasks(filter.propertyIds);
  const propertiesQuery = useProperties();

  const [sort, setSort] = useState<TasksSort>(DEFAULT_TASKS_SORT);
  const [deleteOpen, setDeleteOpen] = useState(false);
  // Страница выбора объектов — строгий черновик (#524): черновик живёт,
  // пока страница смонтирована, история не пишется.
  const [selectOpen, setSelectOpen] = useState(false);
  const [propertyDraft, setPropertyDraft] = useState<ReadonlyArray<string>>([]);

  // Авто-сброс фильтра при 404 (решение 8 #522): объект удалён или доступ
  // отозван — privacy 404, мёртвый фильтр из адреса убирает replace, чтобы
  // «назад» не возвращало на ту же ошибку.
  useEffect(() => {
    if (filter.propertyIds.length === 0) {
      return;
    }
    const error = activeQuery.error ?? completedQuery.error;
    if (error instanceof ApiError && error.status === 404) {
      applyPropertyFilter([], { replace: true });
    }
  }, [filter.propertyIds, activeQuery.error, completedQuery.error, applyPropertyFilter]);

  // Ручная мемоизация активных — требование react-hooks/exhaustive-deps:
  // массив входит в зависимости производных ниже (React Compiler прогоняет
  // сборку, но линт правила хуков на нёё не опирается).
  const active = useMemo(() => activeQuery.data?.items ?? [], [activeQuery.data]);
  const completedItems = completedQuery.data?.items ?? [];
  const completedTotal = completedQuery.data?.total ?? 0;
  const today = activeQuery.data?.today ?? completedQuery.data?.today;

  const propertyOf = (propertyId: string): FeedPropertyRef | undefined => {
    const property = propertiesQuery.data?.find((item) => item.id === propertyId);
    if (property === undefined) {
      return undefined;
    }
    return { role: property.access?.role, status: property.status };
  };

  const complete = useCompleteGlobalTask('complete');
  const uncomplete = useCompleteGlobalTask('uncomplete');
  const completeAll = useCompleteAllGlobalTasks();
  const deleteCompleted = useDeleteCompletedGlobalTasks();

  // Кебаб — когда есть чего им касаться: мутабельные активные строки
  // («Отметить все») или свои выполненные («Удалить выполненные» сносит
  // журнал книги читателя — чужие журналы ему не принадлежат, ADR 0028 и
  // #536). У зрителя чужой ленты кебаб не рисуется — как на объекте.
  const mutableActive = mutableFeedTasks(active, propertyOf);
  const kebabVisible =
    mutableActive.length > 0 ||
    completedItems.some((task) => canMutateFeedTask(task, propertyOf));

  // Секции собираются только при загруженном «сегодня» читателя —
  // границы «Сегодня»/«Завтра» считает сервер (ADR 0048, контракт #521).
  const sections =
    today !== undefined ? groupTasks(active, completedItems, today, sort) : [];

  const showEmpty =
    activeQuery.isSuccess && completedQuery.isSuccess && active.length === 0 && completedTotal === 0;

  const toggleTask = (task: Task): void => {
    if (task.status === 'completed') {
      uncomplete.mutate(task, {
        onError: (error) => notify.scenarios.tasks.uncompleteError(error),
      });
    } else {
      complete.mutate(task, {
        onError: (error) => notify.scenarios.tasks.completeError(error),
      });
    }
  };

  const togglePendingFor = (task: Task): boolean =>
    (complete.isPending || uncomplete.isPending) &&
    (complete.variables?.id === task.id || uncomplete.variables?.id === task.id);

  // Правка касается правила (#502): объектная строка ведёт на объект,
  // безобъектная — на плоский маршрут (#537); выполненные (в том числе
  // журнал удалённого правила) и строки зрителя не открываются.
  const openTaskFor = (task: Task): (() => void) | undefined => {
    if (task.status === 'completed' || task.ruleId === null) {
      return undefined;
    }
    if (!canMutateFeedTask(task, propertyOf)) {
      return undefined;
    }
    const ruleId = task.ruleId;
    if (task.propertyId === null) {
      return () => router.push(ROUTES.taskEdit(ruleId));
    }
    const propertyId = task.propertyId;
    return () => router.push(ROUTES.propertyTaskEdit(propertyId, ruleId));
  };

  const createButton = (
    <IconButton icon={<Add />} label="Создать задачу" disabled aria-disabled />
  );

  // Выбор объекта фильтра (#524): URL не меняется, черновик применяется
  // кнопкой (как выбор объекта контакта #509/#510).
  if (selectOpen) {
    return (
      <TasksPropertySelectPage
        draft={propertyDraft}
        onDraftChange={setPropertyDraft}
        onApply={() => {
          applyPropertyFilter(propertyDraft);
          setSelectOpen(false);
        }}
        onDismiss={() => setSelectOpen(false)}
      />
    );
  }

  return (
    <>
      {/* Хаб-шапка: «крылья» (лого + профиль) и на мобайле, поведение
       * стандартное — в потоке на мобайле, закреплена на десктопе. */}
      <TopNav mobileWings />

      <PageContent>
        <div className="flex items-center justify-between pr-3.5 pl-6">
          <h1 className="text-[28px] font-semibold leading-8 text-content">Задачи</h1>
          {createButton}
        </div>

        {!showEmpty && (
          <div className="mt-6 flex items-center justify-between pr-3.5 pl-6">
            <div className="flex items-center gap-2">
              <PickerMenu title="Сортировать" groups={sortPickerGroups(sort, setSort)}>
                <SortChip sort={sort} />
              </PickerMenu>
              {/* Фильтр по объектам (#524/#547): при выбранном фильтре чип просто
               * активный (синий, макет 1726-86913), подпись всегда «Объект» —
               * названия объектов чип не показывает (правка владельца
               * 2026-09-05). */}
              <ChipButton
                selected={filter.propertyIds.length > 0}
                trailingIcon={<SmallArrowDown />}
                onClick={() => {
                  setPropertyDraft(filter.propertyIds);
                  setSelectOpen(true);
                }}
              >
                Объект
              </ChipButton>
            </div>
            {kebabVisible && (
              <Menu>
                <MenuTrigger asChild>
                  <IconButton icon={<VerticalMenu />} label="Действия со списком" />
                </MenuTrigger>
                <MenuContent>
                  {mutableActive.length > 0 && (
                    <MenuItem
                      icon={<Checkmark />}
                      disabled={completeAll.isPending}
                      onSelect={() => completeAll.mutate(mutableActive)}
                    >
                      Отметить все задачи
                    </MenuItem>
                  )}
                  {completedTotal > 0 && (
                    <MenuItem icon={<TrashBin />} onSelect={() => setDeleteOpen(true)}>
                      Удалить выполненные задачи
                    </MenuItem>
                  )}
                </MenuContent>
              </Menu>
            )}
          </div>
        )}

        <div className="mt-6 flex flex-col gap-6 pb-6">
          {activeQuery.isPending || completedQuery.isPending ? (
            <>
              <TasksSkeleton />
              <TasksSkeleton />
              <TasksSkeleton />
            </>
          ) : activeQuery.isError ? (
            <TasksStateCard
              title="Не удалось загрузить задачи"
              onRetry={() => void activeQuery.refetch()}
            />
          ) : completedQuery.isError ? (
            <TasksStateCard
              title="Не удалось загрузить выполненные"
              onRetry={() => void completedQuery.refetch()}
            />
          ) : showEmpty ? (
            <EmptyState
              imageSrc="/images/tasks/empty-tasks.png"
              imageRounded
              title="Задач нет"
              description="Добавьте задачу — например, позвонить арендатору, вызвать мастера или проверить состояние объекта"
            />
          ) : (
            sections.map((section) => (
              // Пока today не пришёл, sections пуст — заглушка '' не рисуется.
              <FeedSection
                key={sectionKey(section)}
                section={section}
                today={today ?? ''}
                propertyOf={propertyOf}
                completedTotal={completedTotal}
                onToggle={toggleTask}
                toggling={togglePendingFor}
                onOpen={openTaskFor}
              />
            ))
          )}
        </div>
      </PageContent>

      <TasksDeleteCompletedDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        pending={deleteCompleted.isPending}
        onConfirm={() =>
          deleteCompleted.mutate(undefined, {
            onSuccess: () => setDeleteOpen(false),
            onError: (error) => notify.scenarios.tasks.deleteCompletedError(error),
          })
        }
      />
    </>
  );
}

/** Секция ленты: канон объектного экрана, но у строк — строка объекта
 * (имя из глобального листинга #521); у безобъектных её нет (решение 4 #522). */
function FeedSection({
  section,
  today,
  propertyOf,
  completedTotal,
  onToggle,
  toggling,
  onOpen,
}: {
  readonly section: TaskSection;
  readonly today: string;
  readonly propertyOf: (propertyId: string) => FeedPropertyRef | undefined;
  readonly completedTotal: number;
  readonly onToggle: (task: Task) => void;
  readonly toggling: (task: Task) => boolean;
  readonly onOpen: (task: Task) => (() => void) | undefined;
}): JSX.Element {
  const rows = section.tasks.map((task) => (
    <TaskRow
      key={task.id}
      task={task}
      today={today}
      tone={sectionTone(section.kind)}
      canMutate={canMutateFeedTask(task, propertyOf)}
      toggling={toggling(task)}
      onToggle={() => onToggle(task)}
      onOpen={onOpen(task)}
      propertyLine={task.propertyName ?? undefined}
    />
  ));

  if (section.kind === 'completed') {
    return (
      <div data-testid="section-completed" className="mx-6">
        <CollapsibleSection title={section.title} count={completedTotal}>
          <div className="flex flex-col pb-2">{rows}</div>
        </CollapsibleSection>
      </div>
    );
  }
  return (
    <TaskSectionCard title={section.title} testId={`section-${section.kind}`}>
      {rows}
    </TaskSectionCard>
  );
}
