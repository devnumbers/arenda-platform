'use client';

import type { JSX } from 'react';
import { ROUTES } from '@/shared/config/routes';
import { PageContent, TopNav, TopNavBackButton, TopNavTitle } from '@/shared/ui/design';
import { OperationDetailSkeleton, PaymentDetailSkeleton } from './payments-skeletons';

/**
 * Route-loading детализаций платежа и операции (#609): хром подэкрана —
 * вне фазы загрузки (§14, «шапка не прыгает»), контент — скелетоны-архетипы
 * #606. Кнопка «Назад» живая — та же goBack-цель, что у живых экранов
 * (список платежей объекта; фоллбэк — на прямом заходе без истории), рамка
 * кнопки при приходе данных не сдвигается; тайтл данных («Автоплатёж»,
 * дата операции) приходит с экраном.
 */

/** Страница платежа: шапка подэкрана и скелетон карточки правила. */
export function PaymentDetailLoading({
  propertyId,
}: {
  readonly propertyId: string;
}): JSX.Element {
  return (
    <>
      <TopNav leading={<TopNavBackButton fallbackHref={ROUTES.propertyPayments(propertyId)} />}>
        <TopNavTitle title="Платеж" />
      </TopNav>

      <PageContent>
        <PaymentDetailSkeleton />
      </PageContent>
    </>
  );
}

/** Страница операции: шапка подэкрана и скелетон карточки операции. */
export function OperationDetailLoading({
  propertyId,
}: {
  readonly propertyId: string;
}): JSX.Element {
  return (
    <>
      <TopNav leading={<TopNavBackButton fallbackHref={ROUTES.propertyPayments(propertyId)} />}>
        <TopNavTitle title="Операция" />
      </TopNav>

      <PageContent>
        <OperationDetailSkeleton />
      </PageContent>
    </>
  );
}
