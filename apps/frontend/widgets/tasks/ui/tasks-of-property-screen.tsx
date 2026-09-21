'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import { useRouter, usePathname } from 'next/navigation';
import {
  Add,
  ArrowLeft,
  Checkmark,
  TrashBin,
  VerticalMenu,
} from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { notify } from '@/shared/lib/notifications';
import { useProperty } from '@/features/properties';
import {
  DEFAULT_TASKS_SORT,
  groupTasks,
  serializeTasksSortToParams,
  useActiveTasks,
  useCompleteAllTasks,
  useCompletedTasks,
  useCompleteTask,
  useDeleteCompletedTasks,
  useUncompleteTask,
  type TaskSection,
  type TasksSort,
} from '@/features/tasks';
import type { Task } from '@/entities/task';
import {
  Button,
  CollapsibleSection,
  EmptyState,
  IconButton,
  Menu,
  MenuContent,
  MenuItem,
  MenuTrigger,
  PageContent,
  PickerMenu,
  Skeleton,
  StickyBottomBar,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import { TaskRow } from '@/features/tasks';
import { TaskSectionCard } from './task-section-card';
import { TasksDeleteCompletedDialog } from './tasks-delete-completed-dialog';
import { taskSectionTone } from '@/features/tasks';
import { sectionKey } from './tasks-section-utils';
import { SortChip, sortPickerGroups } from './tasks-sort';

/**
 * Экран «Задачи объекта» (#499, Figma 1535-75363/1535-75894/1531-12784):
 * единый список без вкладок — секции-карточки «Просроченные» (красные сроки,
 * всегда сверху) → «Сегодня, D» → «Завтра, D» → даты → «Без даты» →
 * сворачиваемая «Выполненные N» (по умолчанию свёрнута, пустая не
 * показывается). Тап по строке — «Изменить задачу» (#502), тап по кружку —
 * выполнить/снять без уведомлений (решение владельца 2026-09-03). Чип
 * сортировки: ведущая иконка показывает направление (SortingSmallBig —
 * возрастание, SortingBigSmall — убывание); на десктопе открывает меню
 * «Сортировать» (Figma 1603-94487,
 * выбор применяет и закрывает), на мобильной ширине — шит. Выбор сортировки
 * живёт в адресе (?sort=&order=, #785) — переживает перезагрузку. Кебаб ⋮
 * (Figma 1535-77633) —
 * «Отметить все задачи» (все активные, по одному POST, молча — без шитов и
 * уведомлений, решение владельца 2026-09-03) и «Удалить выполненные
 * задачи»; кебаб виден, когда есть активные или выполненные, пункты — по
 * наличию своих строк. Создание — страницей /tasks/new из «+»/«Создать
 * задачу» (#500), редактирование — тапом по строке (#502).
 * Смотрящий читает без мутаций (матрица ADR 0028), мутации глушатся и по
 * архиву (#446).
 */
export function TasksOfPropertyScreen({
  propertyId,
  initialSort,
}: {
  readonly propertyId: string;
  readonly initialSort?: TasksSort;
}): JSX.Element {
  const router = useRouter();
  const pathname = usePathname();
  const propertyQuery = useProperty(propertyId);

  const activeQuery = useActiveTasks(propertyId);
  const completedQuery = useCompletedTasks(propertyId);

  const [sort, setSort] = useState<TasksSort>(initialSort ?? DEFAULT_TASKS_SORT);
  const [deleteOpen, setDeleteOpen] = useState(false);

  // Сортировка живёт в адресе (?sort=&order=, дефолт не пишется — конвенция
  // страницы «Объекты», #785): перезагрузка и шаринг ссылки сохраняют выбор.
  const changeSort = (next: TasksSort): void => {
    setSort(next);
    const query = new URLSearchParams(serializeTasksSortToParams(next)).toString();
    router.replace(query !== '' ? `${pathname}?${query}` : pathname, { scroll: false });
  };

  const complete = useCompleteTask(propertyId);
  const uncomplete = useUncompleteTask(propertyId);
  const completeAll = useCompleteAllTasks(propertyId);
  const deleteCompleted = useDeleteCompletedTasks(propertyId);

  const property = propertyQuery.isSuccess ? propertyQuery.data : undefined;
  const role = property?.access?.role;
  const canMutate =
    property !== undefined && role !== undefined && role !== 'viewer' && property.status !== 'archived';

  const active = activeQuery.data?.items ?? [];
  const completedItems = completedQuery.data?.items ?? [];
  const completedTotal = completedQuery.data?.total ?? 0;
  const today = activeQuery.data?.today;

  // Секции собираются только при загруженном «сегодня» собственника —
  // границы «Сегодня»/«Завтра» считает сервер (ADR 0048).
  const sections =
    today !== undefined ? groupTasks(active, completedItems, today, sort) : [];

  const showEmpty =
    activeQuery.isSuccess && completedQuery.isSuccess && active.length === 0 && completedTotal === 0;

  const toggleTask = (task: Task): void => {
    if (task.status === 'completed') {
      uncomplete.mutate(task.id, {
        onError: (error) => notify.scenarios.tasks.uncompleteError(error),
      });
    } else {
      complete.mutate(task.id, {
        onError: (error) => notify.scenarios.tasks.completeError(error),
      });
    }
  };

  const togglePendingFor = (task: Task): boolean =>
    (complete.isPending || uncomplete.isPending) &&
    (complete.variables === task.id || uncomplete.variables === task.id);

  // Правка касается правила (#502): тап по активной строке открывает форму;
  // выполненные — история со снимком (правило правится из активной строки),
  // у журнала удалённого правила правила нет. Зрителю и в архиве правка
  // недоступна — строки списка не открываются вовсе.
  const openTaskFor = (task: Task): (() => void) | undefined => {
    if (!canMutate || task.status === 'completed' || task.ruleId === null) {
      return undefined;
    }
    const ruleId = task.ruleId;
    return () => router.push(ROUTES.propertyTaskEdit(propertyId, ruleId));
  };

  return (
    <>
      <TopNav
        leading={
          <IconButton
            icon={<ArrowLeft />}
            label="Назад"
            onClick={() => goBack(router, ROUTES.property(propertyId))}
          />
        }
        trailing={
          canMutate ? (
            <IconButton
              icon={<Add />}
              label="Создать задачу"
              onClick={() => router.push(ROUTES.propertyTaskCreate(propertyId))}
            />
          ) : undefined
        }
      >
        <TopNavTitle title="Задачи объекта" />
      </TopNav>

      <PageContent>
        <div className="flex flex-col gap-6 pb-6">
          <div className="flex items-center justify-between pr-3.5 pl-6">
            <PickerMenu title="Сортировать" groups={sortPickerGroups(sort, changeSort)}>
              <SortChip sort={sort} data-testid="tasks-sort-chip" />
            </PickerMenu>
            {canMutate && (active.length > 0 || completedTotal > 0) && (
              <Menu>
                <MenuTrigger asChild>
                  <IconButton icon={<VerticalMenu />} label="Действия со списком" />
                </MenuTrigger>
                <MenuContent>
                  {active.length > 0 && (
                    <MenuItem
                      icon={<Checkmark />}
                      disabled={completeAll.isPending}
                      onSelect={() => completeAll.mutate(active.map((task) => task.id))}
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
              <TaskSection key={sectionKey(section)} section={section}
                today={today ?? ''} canMutate={canMutate}
                completedTotal={completedTotal}
                onToggle={toggleTask} toggling={togglePendingFor}
                onOpen={openTaskFor}
              />
            ))
          )}
        </div>
      </PageContent>

      {canMutate && (
        <StickyBottomBar>
          <Button
            className="w-full"
            onClick={() => router.push(ROUTES.propertyTaskCreate(propertyId))}
          >
            Создать задачу
          </Button>
        </StickyBottomBar>
      )}

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

/** Секция экрана: обычные группы — карточкой, «Выполненные» — сворачиваемой
 * секцией с серверным счётчиком. */
function TaskSection({
  section,
  today,
  canMutate,
  completedTotal,
  onToggle,
  toggling,
  onOpen,
}: {
  readonly section: TaskSection;
  readonly today: string;
  readonly canMutate: boolean;
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
      canMutate={canMutate}
      toggling={toggling(task)}
      onToggle={() => onToggle(task)}
      onOpen={onOpen(task)}
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

/** Карточка состояния с действием — ошибка загрузки с кнопкой «Повторить».
 * Общая с экраном правки задачи (#502). */
export function TasksStateCard({
  title,
  onRetry,
}: {
  readonly title: string;
  readonly onRetry: () => void;
}): JSX.Element {
  return (
    <section className="mx-6 rounded-card bg-surface-muted px-6 py-6">
      <h2 className="text-xl font-semibold leading-6 text-content">{title}</h2>
      <p className="mt-2 text-sm leading-4 text-content-secondary">
        Проверьте подключение и попробуйте еще раз
      </p>
      <div className="mt-4">
        <Button size="small" variant="secondary" onClick={onRetry}>
          Повторить
        </Button>
      </div>
    </section>
  );
}

/** Скелет секции на время загрузки — серая карточка с пульсирующими строками.
 * Общая с экраном правки задачи (#502). */
export function TasksSkeleton(): JSX.Element {
  return (
    <section className="mx-6 rounded-card bg-surface-muted px-6 py-6" aria-hidden>
      <Skeleton className="mb-4 h-6 w-40 bg-surface-muted-hover" />
      <div className="flex flex-col gap-4">
        <Skeleton className="h-11 bg-surface-muted-hover" />
        <Skeleton className="h-11 w-4/5 bg-surface-muted-hover" />
      </div>
    </section>
  );
}
