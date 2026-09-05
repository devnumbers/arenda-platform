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
  TopNavTitle,
} from '@/shared/ui/design';
import { TaskRow } from './task-row';
import { TaskSectionCard } from './task-section-card';
import { TasksDeleteCompletedDialog } from './tasks-delete-completed-dialog';
import { TasksSkeleton, TasksStateCard } from './tasks-of-property-screen';
import { sectionKey, sectionTone } from './tasks-section-utils';
import { SortChip, sortPickerGroups } from './tasks-sort';

/**
 * Экран «Задачи» — глобальная лента (карта #518, тикет #523, Figma
 * 1733-27411/1726-86913/1733-92349): merged-фид читателя из GET /tasks (#521)
 * — свои задачи и задачи видимых объектов, архивы мимо (решение 9 #522).
 * Секции — канон объектного экрана (#499): границы «Сегодня»/«Завтра»
 * считает сервер по календарю читателя, порядок секций приходит плоским по
 * сроку, строки внутри сортируются клиентски (дефолт «Дата, asc» — решение
 * 5 #522; чип «Название» на макете — не дефолт). У объектных строк — строка
 * объекта (HomeMain + имя, propertyName из контракта #521); тап по активной
 * объектной строке — правка на объекте, по безобъектной — плоский маршрут
 * /tasks/{ruleId}/edit (#537, решения 2–3 #522), выполненные некликабельны.
 * Кебаб — «Отметить все» (только мутабельные строки) и «Удалить выполненные»
 * (глобальный DELETE /tasks/completed, #536); мутации выполнения
 * маршрутизируются по срезу (ADR 0052), зритель читает (ADR 0028).
 *
 * Шапка (Figma 1733-92349) закреплена и на мобайле: пока блок заголовка в
 * кадре — «крылья» хаба (лого + профиль, mobileWings); когда заголовок
 * уходит под прокрутку, в шапке появляются компактный заголовок и «+».
 * Чип «Объект» и «+» нарисованы по макету, но пока без действия: фильтр —
 * тикет #524, создание — тикет #525 (принять решение владельца на приёмке).
 */
export function TasksFeedScreen(): JSX.Element {
  const router = useRouter();

  const activeQuery = useGlobalActiveTasks();
  const completedQuery = useGlobalCompletedTasks();
  const propertiesQuery = useProperties();

  const [sort, setSort] = useState<TasksSort>(DEFAULT_TASKS_SORT);
  const [deleteOpen, setDeleteOpen] = useState(false);
  // Компактная шапка: блок заголовка («Задачи» + «+») ушёл из кадра.
  const [headerBlockOut, setHeaderBlockOut] = useState(false);
  const [headerBlockRef, setHeaderBlockRef] = useState<HTMLDivElement | null>(null);

  useEffect(() => {
    if (headerBlockRef === null) {
      return;
    }
    const observer = new IntersectionObserver(
      (entries) => {
        const entry = entries[0];
        if (entry !== undefined) {
          setHeaderBlockOut(!entry.isIntersecting);
        }
      },
      { rootMargin: '-72px 0px 0px 0px' },
    );
    observer.observe(headerBlockRef);
    return () => observer.disconnect();
  }, [headerBlockRef]);

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

  return (
    <>
      <TopNav
        className="fixed inset-x-0 top-0"
        mobileWings={!headerBlockOut}
        trailing={headerBlockOut ? createButton : undefined}
      >
        {headerBlockOut && <TopNavTitle title="Задачи" />}
      </TopNav>

      {/* Шапка закреплена и на мобайле — контент компенсирует её высоту
       * (на десктопе компенсирует ScreenLayout). */}
      <div className="pt-[calc(72px+env(safe-area-inset-top))] desktop:pt-0">
        <PageContent>
          <div
            ref={setHeaderBlockRef}
            className="flex items-center justify-between pr-3.5 pl-6"
          >
            <h1 className="text-[28px] font-semibold leading-8 text-content">Задачи</h1>
            {createButton}
          </div>

          {!showEmpty && (
            <div className="mt-6 flex items-center justify-between pr-3.5 pl-6">
              <div className="flex items-center gap-2">
                <PickerMenu title="Сортировать" groups={sortPickerGroups(sort, setSort)}>
                  <SortChip sort={sort} />
                </PickerMenu>
                {/* Фильтр по объекту — тикет #524: чип по макету, пока без действия. */}
                <ChipButton trailingIcon={<SmallArrowDown />} disabled aria-disabled>
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
      </div>

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
