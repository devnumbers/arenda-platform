'use client';

import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { IconButton, PageContent, TopNav, TopNavTitle } from '@/shared/ui/design';

/**
 * Клиентский хром временной заглушки поиска операций (маршрут ведёт с
 * иконки поиска экрана «Операции объекта», #474): полноценный экран поиска
 * по Figma 1494-61633… — тикет #476; заглушка держит хром маршрута, чтобы
 * иконка не вела в 404.
 */
export function OperationsSearchStubScreen({
  propertyId,
}: {
  readonly propertyId: string;
}): JSX.Element {
  const router = useRouter();

  return (
    <>
      <TopNav
        leading={
          <IconButton
            icon={<ArrowLeft />}
            label="Назад"
            onClick={() => goBack(router, ROUTES.propertyOperations(propertyId))}
          />
        }
      >
        <TopNavTitle title="Поиск операций" />
      </TopNav>
      <PageContent>
        <p className="px-6 pt-6 text-base leading-[18px] text-content-secondary">
          Поиск появится в следующем обновлении
        </p>
      </PageContent>
    </>
  );
}
