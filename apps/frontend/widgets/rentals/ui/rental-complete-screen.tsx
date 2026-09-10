'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { canMutateProperty, useProperty } from '@/features/properties';
import { currentRentalOf, useRentals } from '@/features/rentals';
import type { Rental } from '@/entities/rental';
import { Button, PageContent, Skeleton, TopNav } from '@/shared/ui/design';
import { RentalCompleteFlow } from './rental-complete-flow';

/**
 * Экран «Завершение аренды» (#534): маршрут /properties/[id]/rentals/complete.
 * Работает с текущей (незавершённой) арендой — точка входа, строка
 * «Завершить аренду» секции «Управление» детализации. Гард: загрузка —
 * скелетон; смотрителю — отказ (завершение — Full Access, ADR 0053 §3);
 * без аренды — честный отказ; не начавшаяся (upcoming) не завершается —
 * дата завершения не бывает раньше начала (ADR 0053 §3, сервер — 400).
 * После успешной мутации аренда уходит из незавершённых — экран держит
 * последний смонтированный экземпляр, чтобы мастер дорисовал финал.
 */
export function RentalCompleteScreen({
  propertyId,
}: {
  readonly propertyId: string;
}): JSX.Element {
  const router = useRouter();
  const propertyQuery = useProperty(propertyId);
  const rentalsQuery = useRentals(propertyId);
  const close = (): void => goBack(router, ROUTES.propertyRental(propertyId));

  const loading = propertyQuery.isPending || rentalsQuery.isPending;
  const property = propertyQuery.isSuccess ? propertyQuery.data : undefined;
  const canMutate = canMutateProperty(property);
  const rental = currentRentalOf(rentalsQuery.data ?? []);
  // После успешного завершения аренда уходит из незавершённых — держим
  // последний виденный экземпляр (вывод состояния во время рендера), чтобы
  // смонтированный мастер дорисовал финальный экран.
  const [mountedRental, setMountedRental] = useState<Rental | undefined>(undefined);
  if (rental !== undefined && rental !== mountedRental) {
    setMountedRental(rental);
  }
  const effectiveRental = rental ?? mountedRental;

  return (
    <>
      {loading && (
        <>
          <TopNav />
          <PageContent>
            <div className="flex flex-col gap-4 pt-6">
              <Skeleton className="h-14 w-full" />
              <Skeleton className="h-14 w-full" />
              <Skeleton className="h-14 w-full" />
            </div>
          </PageContent>
        </>
      )}

      {!loading && (propertyQuery.isError || rentalsQuery.isError) && (
        <>
          <TopNav />
          <PageContent>
            <div className="flex flex-col items-center gap-4 pt-6">
              <p className="text-center text-base leading-[18px] text-content-secondary">
                Не удалось загрузить аренду
              </p>
              <Button
                variant="secondary"
                size="small"
                onClick={() => {
                  void propertyQuery.refetch();
                  void rentalsQuery.refetch();
                }}
              >
                Повторить
              </Button>
            </div>
          </PageContent>
        </>
      )}

      {!loading && property !== undefined && !canMutate && (
        <>
          <TopNav />
          <PageContent>
            <p className="pt-6 text-center text-base leading-[18px] text-content-secondary">
              У вас доступ только для просмотра этого объекта
            </p>
          </PageContent>
        </>
      )}

      {!loading && property !== undefined && canMutate && effectiveRental === undefined && (
        <>
          <TopNav />
          <PageContent>
            <p className="pt-6 text-center text-base leading-[18px] text-content-secondary">
              Аренда не найдена
            </p>
          </PageContent>
        </>
      )}

      {!loading &&
        property !== undefined &&
        canMutate &&
        effectiveRental !== undefined &&
        effectiveRental.status === 'upcoming' && (
          <>
            <TopNav />
            <PageContent>
              <p className="pt-6 text-center text-base leading-[18px] text-content-secondary">
                Аренду можно завершить только после её начала
              </p>
            </PageContent>
          </>
        )}

      {!loading &&
        property !== undefined &&
        canMutate &&
        effectiveRental !== undefined &&
        effectiveRental.status !== 'upcoming' && (
          <RentalCompleteFlow
            key={effectiveRental.id}
            rental={effectiveRental}
            propertyName={property.name}
            onClose={close}
          />
        )}
    </>
  );
}
