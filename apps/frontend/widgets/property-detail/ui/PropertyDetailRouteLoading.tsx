'use client';

import type { JSX } from 'react';
import { PageContent, TopNav, TopNavTitle } from '@/shared/ui/design';
import { PropertyDetailLoading } from './PropertyDetailLoading';
import { PropertyDetailBackButton } from './PropertyDetailBackButton';

/**
 * Route-loading детали объекта (#609): хром — вне фазы загрузки (§14),
 * контент — скелетон каркаса #588. Ведущая кнопка — та же хабовая
 * «Назад» на список объектов, что у живой страницы (детерминированный
 * push, не история): пользователь со стрелки не ждёт разного поведения
 * в фазе загрузки. Статус объекта в фазе загрузки неизвестен — заголовок
 * без статусной подписи; статус и его подпись приходят с живой страницей.
 */
export function PropertyDetailRouteLoading(): JSX.Element {
  return (
    <>
      <TopNav leading={<PropertyDetailBackButton />}>
        <TopNavTitle title="Объект" />
      </TopNav>

      <PageContent className="px-6">
        <PropertyDetailLoading />
      </PageContent>
    </>
  );
}
