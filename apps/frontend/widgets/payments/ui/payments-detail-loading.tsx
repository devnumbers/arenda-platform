'use client';

import type { JSX } from 'react';
import { ArrowLeft } from '@/shared/assets/icons';
import { IconButton, PageContent, TopNav, TopNavTitle } from '@/shared/ui/design';
import { OperationDetailSkeleton, PaymentDetailSkeleton } from './payments-skeletons';

/**
 * Route-loading детализаций платежа и операции (#609): хром подэкрана —
 * вне фазы загрузки (§14, «шапка не прыгает»), контент — скелетоны-архетипы
 * #606. Кнопка «Назад» в покое — живой экран подменяет её своей (с роутером)
 * без сдвига; тайтл данных («Автоплатёж», дата операции) приходит с экраном.
 */

/** Страница платежа: шапка подэкрана и скелетон карточки правила. */
export function PaymentDetailLoading(): JSX.Element {
  return (
    <>
      <TopNav leading={<IconButton icon={<ArrowLeft />} label="Назад" />}>
        <TopNavTitle title="Платеж" />
      </TopNav>

      <PageContent>
        <PaymentDetailSkeleton />
      </PageContent>
    </>
  );
}

/** Страница операции: шапка подэкрана и скелетон карточки операции. */
export function OperationDetailLoading(): JSX.Element {
  return (
    <>
      <TopNav leading={<IconButton icon={<ArrowLeft />} label="Назад" />}>
        <TopNavTitle title="Операция" />
      </TopNav>

      <PageContent>
        <OperationDetailSkeleton />
      </PageContent>
    </>
  );
}
