'use client';

import type { JSX } from 'react';
import { ArrowLeft, ChangeVertical, Search } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import {
  ChipButton,
  EmptyState,
  HubCollapseAnchor,
  HubTitle,
  IconButton,
  PageContent,
  SearchField,
  Skeleton,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import { PaymentsRowsSkeleton } from './payments-sections';
import {
  PaymentsGlobalSectionSkeleton,
  PaymentsSearchPill,
} from './payments-global-screen';

/**
 * Route-loading архетипы глобальных платежей (#609): хром экрана — вне
 * фазы загрузки (§7), контент — скелетоны-архетипы #605. Используются как
 * fallback Suspense-границы страницы (окно чанка) и как loading.tsx
 * сегмента (окно RSC). Кнопки в покое — интерактивный экран подменяет их
 * без сдвига.
 */

/** Хаб «Платежи»: шапка с «крыльями», пилюля, три секции-заглушки. */
export function PaymentsLoading(): JSX.Element {
  return (
    <>
      <TopNav
        mobileWings
        collapse={{
          title: 'Платежи',
          search: { href: ROUTES.paymentsSearch, label: 'Найти платёж' },
        }}
      />

      <PageContent>
        <HubCollapseAnchor>
          <HubTitle>Платежи</HubTitle>
        </HubCollapseAnchor>

        <div className="flex flex-col gap-6 px-6 pt-4">
          <PaymentsSearchPill onOpenSearch={() => {}} />

          <PaymentsGlobalSectionSkeleton />
          <PaymentsGlobalSectionSkeleton />
          <PaymentsGlobalSectionSkeleton />
        </div>
      </PageContent>
    </>
  );
}

/** «Избранные платежи»: шапка подэкрана и строки канона PaymentRowButton.
 * Кнопка правки появляется у живого экрана только с данными — в загрузке
 * её нет. */
export function PaymentsFavoritesLoading(): JSX.Element {
  return (
    <>
      <TopNav leading={<IconButton icon={<ArrowLeft />} label="Назад" />}>
        <TopNavTitle title="Избранные платежи" />
      </TopNav>

      <PageContent>
        <div className="flex flex-col">
          <PaymentsRowsSkeleton />
        </div>
      </PageContent>
    </>
  );
}

/** Страница «Объекты»: шапка с лупой и стопки-карточки объектов
 * (Skeleton h-[120px], как в загрузке живого экрана #578). */
export function PaymentsObjectsLoading(): JSX.Element {
  return (
    <>
      <TopNav
        leading={<IconButton icon={<ArrowLeft />} label="Назад" />}
        trailing={<IconButton icon={<Search />} label="Поиск объектов" />}
      >
        <TopNavTitle title="Объекты" />
      </TopNav>

      <PageContent>
        <div className="flex flex-col gap-4 px-6 pt-1">
          <Skeleton className="h-[120px] rounded-card" />
          <Skeleton className="h-[120px] rounded-card" />
        </div>
      </PageContent>
    </>
  );
}

/** Поиск объектов платежей: поисковая шапка канона и подсказка пустого
 * запроса. */
export function PaymentsObjectsSearchLoading(): JSX.Element {
  return (
    <>
      <TopNav
        variant="search"
        leading={<IconButton icon={<ArrowLeft />} label="Назад" />}
      >
        <SearchField aria-label="Найти объект" placeholder="Найти объект" />
      </TopNav>

      <PageContent>
        <EmptyState
          imageSrc="/images/payments/payments-objects-search.png"
          className="py-16"
          description="Введите название или адрес объекта"
        />
      </PageContent>
    </>
  );
}

/** Поиск платежей: поисковая шапка канона и подсказка пустого запроса. */
export function PaymentsSearchLoading(): JSX.Element {
  return (
    <>
      <TopNav
        variant="search"
        leading={<IconButton icon={<ArrowLeft />} label="Назад" />}
      >
        <SearchField aria-label="Поиск платежа" placeholder="Поиск платежа" />
      </TopNav>

      <PageContent>
        <EmptyState
          imageSrc="/images/payments/payments-search.png"
          className="py-16"
          description="Введите название платежа"
        />
      </PageContent>
    </>
  );
}

/** «Просроченные операции»: шапка, чип сортировки (дефолт «Старые»;
 * ?sort=new применит живой экран без сдвига) и строки канона. */
export function PaymentsOverdueLoading(): JSX.Element {
  return (
    <>
      <TopNav leading={<IconButton icon={<ArrowLeft />} label="Назад" />}>
        <TopNavTitle title="Просроченные операции" />
      </TopNav>

      <PageContent>
        <div className="flex flex-col">
          <div className="px-6 pb-3">
            <ChipButton trailingIcon={<ChangeVertical />}>Старые</ChipButton>
          </div>
          <PaymentsRowsSkeleton />
        </div>
      </PageContent>
    </>
  );
}
