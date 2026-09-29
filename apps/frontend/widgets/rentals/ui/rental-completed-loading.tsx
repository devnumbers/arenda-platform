'use client';

import type { JSX } from 'react';
import { ROUTES } from '@/shared/config/routes';
import { PageContent, TopNav, TopNavBackButton, TopNavTitle } from '@/shared/ui/design';
import { RentalCompletedSkeleton } from './rental-skeletons';

/**
 * Route-loading завершённой аренды (#609): хром подэкрана — вне фазы
 * загрузки (§14, «шапка не прыгает»), контент — скелетон-архетип #535.
 * Кнопка «Назад» живая — та же goBack-цель, что у живого экрана
 * («Прошлые аренды» объекта; фоллбэк — на прямом заходе без истории),
 * рамка кнопки при приходе данных не сдвигается. Тайтл парный живому
 * экрану — срок аренды приходит с данными в тело.
 */

/** Карточка прошлой аренды: шапка подэкрана и скелетон завершённой детали. */
export function RentalCompletedLoading({
  propertyId,
}: {
  readonly propertyId: string;
}): JSX.Element {
  return (
    <>
      <TopNav leading={<TopNavBackButton fallbackHref={ROUTES.propertyRentalPast(propertyId)} />}>
        <TopNavTitle title="Аренда" subtitle="Завершена" />
      </TopNav>

      <PageContent>
        <RentalCompletedSkeleton />
      </PageContent>
    </>
  );
}
