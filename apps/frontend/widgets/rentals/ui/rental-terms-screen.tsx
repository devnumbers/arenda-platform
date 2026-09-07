'use client';

import { useRouter } from 'next/navigation';
import type { JSX } from 'react';
import { ArrowLeft } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { currentRentalOf, rentalCommentText, rentalTermsRows, useRentals } from '@/features/rentals';
import { Button, IconButton, PageContent, Skeleton, TopNav, TopNavTitle } from '@/shared/ui/design';

/**
 * «Условия аренды» read-only (#531, Figma 1302:53783/1550:94419): полный
 * просмотр условий текущей аренды — восемь строк, пустые значения «Не
 * указано», комментарий под ними. Правка условий (#532) добавит хвостовую
 * кнопку шапки; завершённая карточка (#535) переиспользует экран.
 */
export function RentalTermsScreen({ propertyId }: { readonly propertyId: string }): JSX.Element {
  const router = useRouter();
  const rentalsQuery = useRentals(propertyId);
  const rental = currentRentalOf(rentalsQuery.data ?? []);

  return (
    <>
      <TopNav
        leading={
          <IconButton
            icon={<ArrowLeft />}
            label="Назад"
            onClick={() => goBack(router, ROUTES.propertyRental(propertyId))}
          />
        }
      >
        <TopNavTitle title="Условия аренды" />
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
                  <div key={row.label} className="flex gap-3">
                    <span className="shrink-0 text-sm leading-4 text-content-secondary">
                      {row.label}
                    </span>
                    <span className="min-w-0 text-sm leading-4 text-content">{row.value}</span>
                  </div>
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
      </PageContent>
    </>
  );
}
