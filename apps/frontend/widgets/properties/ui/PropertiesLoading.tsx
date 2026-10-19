'use client';

import type { JSX } from 'react';
import { Add, SmallArrowDown, SortingSmallBig } from '@/shared/assets/icons';
import { Skeleton, skeletonBlockClass, skeletonRowWidths, type SkeletonRowWidths, Button, HubCollapseAnchor, HubTitle, IconButton, PageContent, SearchPill, TopNav } from '@/shared/ui/design';
import { SORT_FIELD_CHIP_LABEL } from '../lib/property-sort';

/**
 * Скелетон списка «Объектов» — хаба и архива (канон Skeleton, DESIGN.md
 * §7; паритет скелетона и контента #604): копия анатомии PropertyCard —
 * серая карточка radius 24, круг-фото 64, две строки текста; мобайл ≤560 —
 * круг над текстом (зазор 24), 561+ — круг слева вровень (зазор 16),
 * шеврон у правого верхнего края. Блоки внутри серой карточки — тон muted
 * (`bg-surface-muted-hover`), ширины строк — детерминированный цикл
 * `skeletonRowWidths`. Живёт внутри настоящего хрома живой страницы
 * (ветка isLoading) — хром несёт PropertiesHubLoading.
 */
function PropertyCardSkeleton({ widths }: { readonly widths: SkeletonRowWidths }): JSX.Element {
  const muted = skeletonBlockClass('muted');
  return (
    <div
      className="relative flex flex-col gap-6 rounded-card bg-surface-muted p-6 tablet:flex-row tablet:items-start tablet:gap-4"
      aria-hidden
    >
      <Skeleton className={`h-16 w-16 shrink-0 rounded-full ${muted}`} />
      <div className="flex min-w-0 flex-1 flex-col gap-2">
        <Skeleton className={`h-6 ${widths.title} ${muted}`} />
        <Skeleton className={`h-4 ${widths.subtitle} ${muted}`} />
      </div>
      <Skeleton className={`absolute right-6 top-6 h-6 w-6 ${muted}`} />
    </div>
  );
}

export function PropertiesLoading(): JSX.Element {
  const rows = skeletonRowWidths(3);
  return (
    <div className="flex flex-col gap-4" data-testid="properties-loading">
      {rows.map((rowWidths, index) => (
        <PropertyCardSkeleton key={index} widths={rowWidths} />
      ))}
    </div>
  );
}

/**
 * Полный кадр холодной загрузки хаба (план 044, карта #1437): хром экрана
 * — вне фазы загрузки (§7, прецедент TasksLoading) — шапка с «крыльями»,
 * строка заголовка с «+» в покое, пилюля поиска и кнопка-чип сортировки в
 * покое (без PickerMenu — интерактивный экран подменяет их без сдвига),
 * карточки-скелетоны в ритме живого экрана (px-6 pt-4, поиск → чипы →
 * список по 24). Fallback Suspense-границы route-страницы: кадр холодной
 * загрузки неотличим от pending-состояния живого экрана — резолв данных
 * не двигает ни один пиксель.
 */
export function PropertiesHubLoading(): JSX.Element {
  return (
    <>
      <TopNav
        mobileWings
        collapse={{
          title: 'Объекты',
          trailing: <IconButton icon={<Add />} label="Создать объект" />,
        }}
      />

      <PageContent>
        <HubCollapseAnchor>
          <div className="flex h-8 items-center justify-between pr-3.5">
            <HubTitle>Объекты</HubTitle>
            <IconButton icon={<Add />} label="Создать объект" />
          </div>
        </HubCollapseAnchor>

        <div className="flex flex-col px-6 pt-4">
          <SearchPill
            testId="properties-search-pill"
            label="Найти объект"
            onOpenSearch={() => {}}
          />

          <div className="mt-6">
            <Button
              variant="secondary"
              size="small"
              leadingIcon={<SortingSmallBig />}
              trailingIcon={<SmallArrowDown />}
            >
              {SORT_FIELD_CHIP_LABEL.name}
            </Button>
          </div>

          <div className="mt-6">
            <PropertiesLoading />
          </div>
        </div>
      </PageContent>
    </>
  );
}
