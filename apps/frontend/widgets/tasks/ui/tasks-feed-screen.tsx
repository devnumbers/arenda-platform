'use client';

import { useEffect, useMemo, useState } from 'react';
import type { JSX } from 'react';
import { useRouter, usePathname, useSearchParams } from 'next/navigation';
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
  serializeTasksSortToParams,
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
  HubCollapseAnchor,
  IconButton,
  Menu,
  MenuContent,
  MenuItem,
  MenuTrigger,
  PageContent,
  PickerMenu,
  TopNav,
} from '@/shared/ui/design';
import { TaskRow } from '@/features/tasks';
import { TaskSectionCard } from './task-section-card';
import { TasksDeleteCompletedDialog } from './tasks-delete-completed-dialog';
import {
  EMPTY_TASKS_FILTER_DRAFT,
  TasksPropertySelectPage,
  type TasksFilterDraft,
} from './tasks-property-select';
import { TasksFeedSkeleton } from './tasks-skeletons';
import { TasksStateCard } from './tasks-of-property-screen';
import { taskSectionTone } from '@/features/tasks';
import { sectionKey } from './tasks-section-utils';
import { SortChip, sortPickerGroups } from './tasks-sort';

/**
 * Экран «Задачи» — глобальная лента (карта #518, тикет #523, Figma
 * 1733-27411/1726-86913): merged-фид читателя из GET /tasks (#521) — свои
 * задачи и задачи видимых объектов, архивы мимо (решение 9 #522). Секции —
 * канон объектного экрана (#499): границы «Сегодня»/«Завтра» считает сервер
 * по календаре читателя, порядок секций приходит плоским по сроку, строки
 * внутри сортируются клиентски (дефолт «Дата, asc» — решение 5 #522; чип
 * «Название» на макете — не дефолт). Выбор сортировки живёт в адресе
 * (?sort=&order=, #785) — переживает перезагрузку. У объектных строк — строка объекта
 * (HomeMainSmall + имя, propertyName из контракта #521); тап по активной
 * объектной строке — правка на объекте, по безобъектной — плоский маршрут
 * /tasks/{ruleId}/edit (#537, решения 2–3 #522), выполненные некликабельны.
 * Кебаб — «Отметить все» (только мутабельные строки) и «Удалить выполненные»
 * (глобальный DELETE /tasks/completed, #536); мутации выполнения
 * маршрутизируются по срезу (ADR 0052), зритель читает (ADR 0028).
 *
 * Шапка — стандартный TopNav хаба с «крыльями» и на мобайле (`mobileWings`,
 * Figma 1733-27411); смена на компактный заголовок по прокрутке (1733-92349)
 * отложена — решение владельца 2026-09-05. Чип «Объект» — фильтр ленты
 * (#524/#547, Figma 1726-88880/1726-86913): выбранные объекты и «Общие
 * задачи» живут в адресе (?property=<id>[,<id>…], ?withoutProperty=1 —
 * union, решение владельца 2026-09-07), страница выбора — строгий черновик
 * без истории, при 404 фильтр сбрасывается сам (решение 8 #522). Подпись
 * чипа всегда «Объект», активное состояние — признак включённого фильтра
 * (решение владельца 2026-09-05, перекрывает «чип = имя объекта» из
 * решения 7 #522). «+» — создание задачи (#525): та же форма, что на
 * объекте, вход без предвыбранного объекта — маршрут /tasks/new.
 */
export function TasksFeedScreen({
  initialSort,
}: {
  readonly initialSort?: TasksSort;
}): JSX.Element {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const { filter, applyFeedFilter } = useTasksFeedFilter();
  const activeQuery = useGlobalActiveTasks(filter.propertyIds, filter.withoutProperty);
  const completedQuery = useGlobalCompletedTasks(filter.propertyIds, filter.withoutProperty);
  const propertiesQuery = useProperties();

  const [sort, setSort] = useState<TasksSort>(initialSort ?? DEFAULT_TASKS_SORT);
  const [deleteOpen, setDeleteOpen] = useState(false);
  // Страница выбора объектов — строгий черновик (#524): черновик живёт,
  // пока страница смонтирована, история не пишется.
  const [selectOpen, setSelectOpen] = useState(false);
  const [filterDraft, setFilterDraft] = useState<TasksFilterDraft>(EMPTY_TASKS_FILTER_DRAFT);

  // Сортировка живёт в адресе (?sort=&order=, дефолт не пишется — конвенция
  // страницы «Объекты», #785): перезагрузка и шаринг ссылки сохраняют выбор.
  // Пишется поверх текущих параметров (как applyFeedFilter), чтобы не
  // затирать фильтр ленты #524; дефолтные значения параметров снимаются.
  const changeSort = (next: TasksSort): void => {
    setSort(next);
    const params = new URLSearchParams(searchParams);
    for (const [name, value] of Object.entries(serializeTasksSortToParams(next))) {
      params.set(name, value);
    }
    for (const name of ['sort', 'order']) {
      if (!params.get(name)) {
        params.delete(name);
      }
    }
    const query = params.toString();
    router.replace(query !== '' ? `${pathname}?${query}` : pathname, { scroll: false });
  };

  // Авто-сброс фильтра при 404 (решение 8 #522): объект удалён или доступ
  // отозван — privacy 404, мёртвый фильтр из адреса убирает replace, чтобы
  // «назад» не возвращало на ту же ошибку.
  const { propertyIds, withoutProperty } = filter;
  useEffect(() => {
    if (propertyIds.length === 0 && !withoutProperty) {
      return;
    }
    const error = activeQuery.error ?? completedQuery.error;
    if (error instanceof ApiError && error.status === 404) {
      applyFeedFilter(EMPTY_TASKS_FILTER_DRAFT, { replace: true });
    }
  }, [propertyIds, withoutProperty, activeQuery.error, completedQuery.error, applyFeedFilter]);

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
    <IconButton
      icon={<Add />}
      label="Создать задачу"
      onClick={() => router.push(ROUTES.taskCreate)}
    />
  );

  // Выбор фильтра (#524, «Общие задачи» — решение владельца 2026-09-07):
  // URL не меняется, пока страница открыта, черновик применяется кнопкой
  // (как выбор объекта контакта #509/#510).
  if (selectOpen) {
    return (
      <TasksPropertySelectPage
        draft={filterDraft}
        onDraftChange={setFilterDraft}
        onApply={() => {
          applyFeedFilter(filterDraft);
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
      <TopNav mobileWings collapse={{ title: 'Задачи', trailing: createButton }} />

      <PageContent>
        <HubCollapseAnchor>
          <div className="flex items-center justify-between pr-3.5 pl-6">
            <h1 className="m-0 text-[28px] font-semibold leading-8 text-content">Задачи</h1>
            {createButton}
          </div>

          {!showEmpty && (
          <div className="mt-4 flex items-center justify-between pr-3.5 pl-6">
            <div className="flex items-center gap-2">
              <PickerMenu title="Сортировать" groups={sortPickerGroups(sort, changeSort)}>
                <SortChip sort={sort} data-testid="tasks-sort-chip" />
              </PickerMenu>
              {/* Фильтр (#524/#547, «Общие задачи» — решение владельца
               * 2026-09-07): при любом непустом фильтре чип активный
               * (синий, макет 1726-86913), подпись всегда «Объект» —
               * названия чип не показывает (правка владельца 2026-09-05). */}
              <ChipButton
                selected={filter.propertyIds.length > 0 || filter.withoutProperty}
                trailingIcon={<SmallArrowDown />}
                onClick={() => {
                  setFilterDraft(filter);
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
        </HubCollapseAnchor>

        <div className="mt-6 flex flex-col gap-6 pb-6">
          {activeQuery.isPending || completedQuery.isPending ? (
            // Паритет §7: секции-карточки со строками задач и свёрнутые
            // «Выполненные»; чипы выше — вне фазы загрузки.
            <TasksFeedSkeleton />
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
      tone={taskSectionTone(section.kind)}
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
