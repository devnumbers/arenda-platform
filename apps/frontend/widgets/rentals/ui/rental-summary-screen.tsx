'use client';

import { useEffect, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { Cancel } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { useProperty } from '@/features/properties';
import { useRentalSummary, useRentals } from '@/features/rentals';
import {
  Button,
  EmptyState,
  IconButton,
  PageContent,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import { RentalSummaryContent } from './rental-summary-content';
import { useContact } from '@/features/contacts';
import { RentalSummarySkeleton } from './rental-skeletons';

/**
 * «Итоги аренды» завершённой (#535, Figma 1795:101495): read-only повтор
 * шага итогов мастера завершения (#534) на записанных данных — период
 * до фактической даты завершения, финансы объекта за период, возврат
 * залога и данные аренды. Открывается кнопкой «Посмотреть итоги»
 * завершённой детализации; финансы считаются только при платежах —
 * кнопка на детализации показывается вместе с ними.
 */
export function RentalSummaryScreen({
  propertyId,
  rentalId,
}: {
  readonly propertyId: string;
  readonly rentalId: string;
}): JSX.Element {
  const router = useRouter();
  const propertyQuery = useProperty(propertyId);
  const rentalsQuery = useRentals(propertyId);
  const rental = rentalsQuery.data?.find((item) => item.id === rentalId);
  // Фото арендатора — из его карточки контакта (ADR 0065, решение #1286);
  // без арендатора запрос спит (enabled: Boolean(contactId)).
  const tenantContactQuery = useContact(rental?.tenant?.contactId ?? '');
  const completedDate = rental?.completedDate ?? null;
  const summaryQuery = useRentalSummary(
    propertyId,
    rentalId,
    completedDate ?? rental?.startDate ?? '',
  );

  useEffect(() => {
    if (rentalsQuery.isSuccess && rental !== undefined && rental.status !== 'completed') {
      router.replace(ROUTES.propertyRental(propertyId));
    }
  }, [rentalsQuery.isSuccess, rental, router, propertyId]);

  const back = (): void =>
    goBack(
      router,
      rental !== undefined
        ? ROUTES.propertyRentalCompleted(propertyId, rental.id)
        : ROUTES.propertyRentalPast(propertyId),
    );

  const loading =
    rentalsQuery.isPending ||
    propertyQuery.isPending ||
    (rental !== undefined && completedDate !== null && summaryQuery.isPending);

  return (
    <>
      <TopNav
        leading={<IconButton icon={<Cancel />} label="Закрыть" onClick={back} />}
      >
        <TopNavTitle title="Итоги аренды" />
      </TopNav>

      <PageContent>
        {loading && <RentalSummarySkeleton />}

        {!loading && (rentalsQuery.isError || propertyQuery.isError || summaryQuery.isError) && (
          <div className="flex flex-col items-center gap-4 pt-6">
            <p className="text-center text-base leading-[18px] text-content-secondary">
              Не удалось загрузить итоги
            </p>
            <Button
              variant="secondary"
              size="small"
              onClick={() => {
                void rentalsQuery.refetch();
                void propertyQuery.refetch();
                void summaryQuery.refetch();
              }}
            >
              Повторить
            </Button>
          </div>
        )}

        {!loading &&
          !rentalsQuery.isError &&
          !propertyQuery.isError &&
          !summaryQuery.isError &&
          (rental === undefined ||
            (rental.status === 'completed' && completedDate === null)) && (
            <EmptyState
              imageSrc="/images/rentals/rental-hero.png"
              title="Аренда не найдена"
              description="Возможно, она была удалена"
            />
          )}

        {!loading &&
          !rentalsQuery.isError &&
          !propertyQuery.isError &&
          !summaryQuery.isError &&
          rental !== undefined &&
          rental.status === 'completed' &&
          summaryQuery.isSuccess &&
          completedDate !== null && (
            <RentalSummaryContent
              rental={rental}
              summary={summaryQuery.data}
              endDate={completedDate}
              propertyName={propertyQuery.data.name}
              propertyType={propertyQuery.data.type}
              propertyPhotoUrl={propertyQuery.data.photoUrl}
              tenantPhotoUrl={tenantContactQuery.data?.photoUrl}
              depositReturnKopecks={rental.depositReturnKopecks ?? 0}
              depositReturnComment={rental.depositReturnComment ?? ''}
            />
          )}
      </PageContent>
    </>
  );
}
