'use client';

import type { JSX } from 'react';
import { ArrowLeft } from '@/shared/assets/icons';
import { IconButton, PageContent, TopNav, TopNavTitle } from '@/shared/ui/design';
import { RentalCompletedSkeleton } from './rental-skeletons';

/**
 * Route-loading завершённой аренды (#609): хром подэкрана — вне фазы
 * загрузки (§14, «шапка не прыгает»), контент — скелетон-архетип #535.
 * Тайтл парный живому экрану — срок аренды приходит с данными в тело.
 */

/** Карточка прошлой аренды: шапка подэкрана и скелетон завершённой детали. */
export function RentalCompletedLoading(): JSX.Element {
  return (
    <>
      <TopNav leading={<IconButton icon={<ArrowLeft />} label="Назад" />}>
        <TopNavTitle title="Аренда" subtitle="Завершена" />
      </TopNav>

      <PageContent>
        <RentalCompletedSkeleton />
      </PageContent>
    </>
  );
}
