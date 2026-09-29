'use client';

import type { JSX } from 'react';
import { ROUTES } from '@/shared/config/routes';
import { PageContent, TopNav, TopNavBackButton, TopNavTitle } from '@/shared/ui/design';
import { PropertyDetailLoading } from './PropertyDetailLoading';

/**
 * Route-loading детали объекта (#609): хром — вне фазы загрузки (§14),
 * контент — скелетон каркаса #588. Ведущая кнопка фоллбэка — нейтральная
 * «Назад»: слот живой страницы тарифозависим (звезда апселла у базового,
 * решение владельца 10.09), а статус объекта в фазе загрузки неизвестен —
 * статичная кнопка не мигает при приходе данных; статус и его подпись
 * приходят с живой страницей.
 */
export function PropertyDetailRouteLoading(): JSX.Element {
  return (
    <>
      <TopNav leading={<TopNavBackButton fallbackHref={ROUTES.properties} />}>
        <TopNavTitle title="Объект" />
      </TopNav>

      <PageContent className="px-6">
        <PropertyDetailLoading />
      </PageContent>
    </>
  );
}
