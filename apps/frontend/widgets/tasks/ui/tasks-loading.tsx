'use client';

import type { JSX } from 'react';
import { Add, Cancel, SmallArrowDown } from '@/shared/assets/icons';
import { DEFAULT_TASKS_SORT } from '@/features/tasks';
import {
  ChipButton,
  HubCollapseAnchor,
  IconButton,
  PageContent,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import {
  TaskCreateStepSkeleton,
  TaskEditFormSkeleton,
  TasksFeedSkeleton,
} from './tasks-skeletons';
import { SortChip } from './tasks-sort';

/**
 * Route-loading архетипы зоны задач (#609): хром экрана (шапка, чипы,
 * заголовок) — вне фазы загрузки (§7), контент — скелетоны-архетипы
 * #605/#606/#607. Используются и как fallback Suspense-границы страницы
 * (окно загрузки чанка), и как loading.tsx сегмента (окно RSC) — кадр
 * перехода неотличим от pending-состояния живого экрана. Кнопки в покое:
 * интерактивный экран подменяет их без сдвига.
 */

/** Лента задач: хаб-шапка с «крыльями», чипы, скелетон ленты. */
export function TasksLoading(): JSX.Element {
  return (
    <>
      <TopNav
        mobileWings
        collapse={{
          title: 'Задачи',
          trailing: <IconButton icon={<Add />} label="Создать задачу" />,
        }}
      />

      <PageContent>
        <HubCollapseAnchor>
          <div className="flex items-center justify-between pr-3.5 pl-6">
            <h1 className="m-0 text-[28px] font-semibold leading-8 text-content">Задачи</h1>
            <IconButton icon={<Add />} label="Создать задачу" />
          </div>

          <div className="mt-4 flex items-center justify-between pr-3.5 pl-6">
            <div className="flex items-center gap-2">
              <SortChip sort={DEFAULT_TASKS_SORT} />
              <ChipButton trailingIcon={<SmallArrowDown />}>Объект</ChipButton>
            </div>
          </div>
        </HubCollapseAnchor>

        <div className="mt-6 flex flex-col gap-6 pb-6">
          <TasksFeedSkeleton />
        </div>
      </PageContent>
    </>
  );
}

/** Шаг 1 создания задачи: поле «Задача» с полем «Объект» глобального
 * входа (#525). */
export function TaskCreateLoading(): JSX.Element {
  return (
    <>
      <TopNav leading={<IconButton icon={<Cancel />} label="Закрыть" />}>
        <TopNavTitle title="Создать задачу" />
      </TopNav>

      <PageContent>
        <TaskCreateStepSkeleton step={1} withObject />
      </PageContent>
    </>
  );
}

/** Форма правки задачи: каркас «Задача» (#606). */
export function TaskEditLoading(): JSX.Element {
  return (
    <>
      <TopNav leading={<IconButton icon={<Cancel />} label="Закрыть" />}>
        <TopNavTitle title="Изменить задачу" />
      </TopNav>

      <PageContent>
        <div className="flex flex-col gap-8">
          <TaskEditFormSkeleton />
        </div>
      </PageContent>
    </>
  );
}
