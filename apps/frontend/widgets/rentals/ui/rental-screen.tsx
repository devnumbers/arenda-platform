'use client';

import { useRouter } from 'next/navigation';
import type { JSX } from 'react';
import { ArrowLeft } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { useProperty } from '@/features/properties';
import { propertyPermissions } from '@/entities/property';
import { currentRentalOf, useRentals } from '@/features/rentals';
import {
  Button,
  EmptyState,
  IconButton,
  PageContent,
  StickyBottomBar,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import { RentalDetailBody } from './rental-detail-body';
import { RentalDetailSkeleton } from './rental-skeletons';

/**
 * Экран «Аренда» (#531, Figma 1425:55656/1232:61259): маршрут
 * /properties/[id]/rentals. Без незавершённой аренды — пустое состояние
 * с кнопкой «Добавить аренду» в нижней панели (кнопка скрыта у смотрящего —
 * создание только Full Access, ADR 0053 §3) и, если завершённые есть,
 * входом в «Прошлые аренды» (#535); с незавершённой — детализация первой
 * аренды списка.
 */
export function RentalScreen({ propertyId }: { readonly propertyId: string }): JSX.Element {
  const router = useRouter();
  const propertyQuery = useProperty(propertyId);
  const rentalsQuery = useRentals(propertyId);

  const loading = propertyQuery.isPending || rentalsQuery.isPending;
  const failed = propertyQuery.isError || rentalsQuery.isError;
  const property = propertyQuery.isSuccess ? propertyQuery.data : undefined;
  const canMutate = propertyPermissions(property).canEdit;

  // Незавершённая аренда всегда первая (ADR 0053 §4); завершённые —
  // материал «Прошлых аренд» (#535), экраном текущей не являются.
  const rentals = rentalsQuery.data ?? [];
  const currentRental = currentRentalOf(rentals);
  const hasCompletedRentals = rentals.some((rental) => rental.status === 'completed');

  return (
    <>
      <TopNav
        leading={
          <IconButton
            icon={<ArrowLeft />}
            label="Назад"
            onClick={() => goBack(router, ROUTES.property(propertyId))}
          />
        }
      >
        <TopNavTitle title="Аренда" />
      </TopNav>

      {loading && (
        <PageContent>
          <RentalDetailSkeleton />
        </PageContent>
      )}

      {!loading && failed && (
        <PageContent>
          <div className="flex flex-col items-center gap-4 pt-6">
            <p className="text-center text-base leading-[18px] text-content-secondary">
              Не удалось загрузить аренду
            </p>
            <Button
              variant="secondary"
              size="small"
              onClick={() => {
                void rentalsQuery.refetch();
                void propertyQuery.refetch();
              }}
            >
              Повторить
            </Button>
          </div>
        </PageContent>
      )}

      {!loading && !failed && currentRental === undefined && (
        <>
          <PageContent>
            <EmptyState
              imageSrc="/images/rentals/empty-rental.png"
              title="Аренда не создана"
              description="Добавьте аренду и отслеживайте оплату"
            />
          </PageContent>
          {(canMutate || hasCompletedRentals) && (
            <StickyBottomBar>
              {canMutate && (
                <Button
                  className="w-full"
                  onClick={() => router.push(ROUTES.propertyRentalNew(propertyId))}
                >
                  Добавить аренду
                </Button>
              )}
              {hasCompletedRentals && (
                <Button
                  variant="secondary"
                  className="w-full"
                  onClick={() => router.push(ROUTES.propertyRentalPast(propertyId))}
                >
                  Прошлые аренды
                </Button>
              )}
            </StickyBottomBar>
          )}
        </>
      )}

      {!loading && !failed && currentRental !== undefined && (
        <RentalDetailBody
          key={currentRental.id}
          propertyId={propertyId}
          rental={currentRental}
          canMutate={canMutate}
          hasCompletedRentals={hasCompletedRentals}
        />
      )}
    </>
  );
}
