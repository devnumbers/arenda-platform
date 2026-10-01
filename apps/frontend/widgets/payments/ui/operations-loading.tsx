'use client';

import type { JSX } from 'react';
import { Add, ArrowLeft, BoldObjects, Cancel, Check, Search } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { operationsPeriodChipLabel, operationsPropertyChipLabel } from '@/features/payments';
import { OperationsPeriodChip } from './operations-filter-chips';
import {
  Button,
  Checkbox,
  EmptyState,
  HubCollapseAnchor,
  HubTitle,
  IconButton,
  ListRow,
  PageContent,
  SearchField,
  StickyBottomBar,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import {
  OperationsCategoriesSkeleton,
  OperationsDateFeedSkeleton,
  OperationsObjectsSkeleton,
  OperationsSummarySkeleton,
} from './operations-skeletons';
import { OperationAmountStepSkeleton } from './payments-skeletons';
import { OperationsFilterChips } from './operations-filter-chips';
import { OperationsSearchPill } from './operations-global-screen';
import { SelectAvatar } from './operations-objects-select-screen';
import { WizardBottomBar } from './payment-create-wizard/wizard-chrome';

/**
 * Route-loading архетипы глобальных операций (#609): хром экрана (шапка,
 * чипы фильтров, заголовок) — вне фазы загрузки (§7), контент —
 * скелетоны-архетипы #605/#607. Используются как fallback Suspense-
 * границы страницы (окно чанка) и как loading.tsx сегмента (окно RSC).
 * Чипы — дефолтные лейблы: фильтры из адреса применит живой экран без
 * сдвига геометрии. Кнопки в покое — интерактивный экран подменяет их
 * без сдвига.
 */

/** Дефолтные чипы фильтров глобальной ленты (весь период — нейтральный
 * «Период» #670, без срезов). */
function OperationsFilterChipsLoading(): JSX.Element {
  return (
    <OperationsFilterChips
      className="px-6"
      periodLabel={operationsPeriodChipLabel(null)}
      periodActive={false}
      propertyLabel={operationsPropertyChipLabel([])}
      propertyActive={false}
      categoriesLabel="Все категории"
      categoriesActive={false}
      onOpenPeriod={() => {}}
      onOpenCategories={() => {}}
    />
  );
}

/** Хаб «Операции»: шапка с «крыльями», пилюля, чипы, сводка и лента дат. */
export function OperationsLoading(): JSX.Element {
  return (
    <>
      <TopNav
        mobileWings
        collapse={{
          title: 'Операции',
          search: { href: ROUTES.operationsSearch, label: 'Найти операцию' },
          trailing: <IconButton icon={<Add />} label="Добавить операцию" />,
        }}
      />

      <PageContent>
        <HubCollapseAnchor>
          <div className="flex h-8 items-center justify-between pr-3.5">
            <HubTitle>Операции</HubTitle>
            <IconButton icon={<Add />} label="Добавить операцию" />
          </div>
          <div className="mt-4 px-6">
            <OperationsSearchPill onOpenSearch={() => {}} />
          </div>
        </HubCollapseAnchor>

        <div className="mt-4 flex flex-col gap-6">
          <OperationsFilterChipsLoading />

          <OperationsSummarySkeleton cards={2} />
          <OperationsDateFeedSkeleton />
        </div>
      </PageContent>
    </>
  );
}

/** Направление «Расходы»/«Доходы»: шапка подэкрана с лупой и «+», одна
 * карточка сводки и лента дат. */
export function OperationsDirectionLoading({
  title,
}: {
  readonly title: 'Расходы' | 'Доходы';
}): JSX.Element {
  return (
    <>
      <TopNav
        leading={<IconButton icon={<ArrowLeft />} label="Назад" />}
        trailing={
          <>
            <IconButton icon={<Search />} label="Найти операцию" />
            <IconButton icon={<Add />} label="Добавить операцию" />
          </>
        }
      >
        <TopNavTitle title={title} />
      </TopNav>

      <PageContent>
        <div className="flex flex-col gap-6 pt-4">
          <OperationsFilterChipsLoading />

          <OperationsSummarySkeleton cards={1} />
          <OperationsDateFeedSkeleton />
        </div>
      </PageContent>
    </>
  );
}

/** Выборщик категорий: шапка выборщика и чипы контекста, строки-радио. */
export function OperationsCategoriesLoading(): JSX.Element {
  return (
    <>
      <TopNav leading={<IconButton icon={<Cancel />} label="Закрыть" />}>
        <TopNavTitle title="Выбрать категорию" />
      </TopNav>

      <PageContent>
        <div className="flex flex-col gap-6 pt-4">
          <div className="flex flex-wrap gap-1.5 px-6">
            <OperationsPeriodChip period={null} />
            <span
              aria-hidden
              className="inline-flex h-11 items-center rounded-pill bg-surface-muted px-5 text-sm font-medium text-content"
            >
              Все категории
            </span>
          </div>

          <OperationsCategoriesSkeleton />
        </div>
      </PageContent>
    </>
  );
}

/** Выборщик объектов: шапка выборщика, ряд «Все объекты», строки
 * объектов. */
export function OperationsObjectsLoading(): JSX.Element {
  return (
    <>
      <TopNav
        leading={<IconButton icon={<Cancel />} label="Закрыть" />}
        trailing={
          <IconButton icon={<Check />} label="Применить выбранные объекты" />
        }
      >
        <TopNavTitle title="Выбрать объект" />
      </TopNav>

      <PageContent>
        <div>
          <ListRow
            leading={<SelectAvatar fallback={<BoldObjects className="h-6 w-6 text-[#D3D7D9]" />} />}
            title="Все объекты"
            trailing={<Checkbox aria-label="Все объекты" checked />}
          />
          <div aria-hidden className="mx-6 h-px bg-surface-muted" />
          <OperationsObjectsSkeleton />
        </div>
      </PageContent>
    </>
  );
}

/** Поиск операций: поисковая шапка канона #543 и подсказка пустого
 * запроса — первый кадр живого экрана до ввода. */
export function OperationsSearchLoading(): JSX.Element {
  return (
    <>
      <TopNav
        variant="search"
        leading={<IconButton icon={<ArrowLeft />} label="Назад" />}
      >
        <SearchField aria-label="Найти операцию" placeholder="Найти операцию" />
      </TopNav>

      <PageContent>
        <EmptyState
          imageSrc="/images/payments/operations-search.png"
          className="py-16"
          description="Введите название операции, сумму, категорию"
        />
      </PageContent>
    </>
  );
}

/** Визард одиночной операции: хром шага «Добавить операцию» и нижняя
 * панель «Продолжить» — вне фазы загрузки, как в загрузке самого
 * визарда (#607). */
export function OperationCreateLoading(): JSX.Element {
  return (
    <>
      <TopNav leading={<IconButton icon={<Cancel />} label="Закрыть" />}>
        <TopNavTitle title="Добавить операцию" />
      </TopNav>

      <PageContent className="pt-0">
        <OperationAmountStepSkeleton />
      </PageContent>

      <StickyBottomBar>
        <WizardBottomBar>
          <Button className="w-full" disabled>
            Продолжить
          </Button>
        </WizardBottomBar>
      </StickyBottomBar>
    </>
  );
}
