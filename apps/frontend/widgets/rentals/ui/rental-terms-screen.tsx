'use client';

import { useRouter } from 'next/navigation';
import type { JSX } from 'react';
import { ArrowLeft, Edit } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { currentRentalOf, rentalCommentText, rentalTermsRows, useRentals } from '@/features/rentals';
import { Button, EmptyState, IconButton, PageContent, Skeleton, TopNav, TopNavTitle } from '@/shared/ui/design';
import { TermRow } from './term-row';

/**
 * «Условия аренды» read-only (#531, Figma 1302:53783/1550:94419): полный
 * просмотр условий — восемь строк, пустые значения «Не указано»,
 * комментарий под ними. Без rentalId — условия текущей аренды, карандаш
 * в шапке ведёт на правку (#532). С rentalId — условия завершённой аренды
 * (#535, 1550:94804): подзаголовок «В архиве», правки нет.
 */
export function RentalTermsScreen({
  propertyId,
  rentalId,
}: {
  readonly propertyId: string;
  readonly rentalId?: string;
}): JSX.Element {
  const router = useRouter();
  const rentalsQuery = useRentals(propertyId);
  const rental =
    rentalId === undefined
      ? currentRentalOf(rentalsQuery.data ?? [])
      : rentalsQuery.data?.find((item) => item.id === rentalId);
  const archived = rentalId !== undefined;

  return (
    <>
      <TopNav
        leading={
          <IconButton
            icon={<ArrowLeft />}
            label="Назад"
            onClick={() =>
              goBack(
                router,
                archived
                  ? ROUTES.propertyRentalCompleted(propertyId, rentalId)
                  : ROUTES.propertyRental(propertyId),
              )
            }
          />
        }
        trailing={
          archived || rental === undefined ? undefined : (
            <IconButton
              icon={<Edit />}
              label="Изменить условия"
              onClick={() => router.push(ROUTES.propertyRentalTermsEdit(propertyId))}
            />
          )
        }
      >
        <TopNavTitle
          title="Условия аренды"
          subtitle={archived ? 'В архиве' : undefined}
        />
      </TopNav>

      <PageContent>
        {rentalsQuery.isPending && (
          <div className="flex flex-col gap-4 pt-6">
            <Skeleton className="h-14 w-full" />
            <Skeleton className="h-14 w-full" />
            <Skeleton className="h-14 w-full" />
          </div>
        )}

        {rentalsQuery.isError && (
          <div className="flex flex-col items-center gap-4 pt-6">
            <p className="text-center text-base leading-[18px] text-content-secondary">
              Не удалось загрузить аренду
            </p>
            <Button variant="secondary" size="small" onClick={() => void rentalsQuery.refetch()}>
              Повторить
            </Button>
          </div>
        )}

        {!rentalsQuery.isPending && !rentalsQuery.isError && rental !== undefined && (
          <div className="px-6">
            <section className="flex flex-col gap-6 rounded-card bg-surface-muted p-6">
              <div className="flex flex-col gap-2">
                {rentalTermsRows(rental, rental.today).map((row) => (
                  <TermRow key={row.label} label={row.label} value={row.value} />
                ))}
              </div>
              <div className="flex flex-col gap-1">
                <span className="text-sm leading-4 text-content-secondary">Комментарий</span>
                <span className="min-w-0 whitespace-pre-line text-sm leading-4 text-content">
                  {rentalCommentText(rental.comment)}
                </span>
              </div>
            </section>
          </div>
        )}
        {!rentalsQuery.isPending && !rentalsQuery.isError && rental === undefined && archived && (
          // Ссылка на условия несуществующей/незавершённой аренды — без контента.
          <EmptyState
            imageSrc="/images/rentals/rental-hero.png"
            title="Аренда не найдена"
            description="Возможно, она была удалена"
          />
        )}
      </PageContent>
    </>
  );
}
