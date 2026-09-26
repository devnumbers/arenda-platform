'use client';

import { useRouter } from 'next/navigation';
import type { JSX } from 'react';
import { ArrowLeft, BoldUser } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import {
  completedRentalsOf,
  pastRentalCardTitle,
  pastRentalRows,
  rentalTenantTitle,
  useRentals,
} from '@/features/rentals';
import type { Rental } from '@/entities/rental';
import { PaymentRowButton } from '@/entities/payment';
import { Button, CircleIcon, EmptyState, IconButton, PageContent, Skeleton, TopNav, TopNavTitle } from '@/shared/ui/design';
import { RentalGroup } from './rental-group';
import { TermRows } from './term-row';

/**
 * Экран «Прошлые аренды» (#535, Figma 1302:52462/1302:53003): карточки
 * завершённых аренд объекта — срок заголовком («24 месяца»), плата,
 * начало, окончание и строка арендатора; тап по карточке — завершённая
 * детализация. Без завершённых — пустое состояние с единой картинкой-ключом
 * (решение владельца 2026-09-07). Порядок карточек — серверный: по дате
 * завершения, свежие сверху (ADR 0053 §4).
 */
export function RentalPastScreen({ propertyId }: { readonly propertyId: string }): JSX.Element {
  const router = useRouter();
  const rentalsQuery = useRentals(propertyId);
  const completed = completedRentalsOf(rentalsQuery.data ?? []);

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
        <TopNavTitle title="Прошлые аренды" />
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
              Не удалось загрузить прошлые аренды
            </p>
            <Button variant="secondary" size="small" onClick={() => void rentalsQuery.refetch()}>
              Повторить
            </Button>
          </div>
        )}

        {!rentalsQuery.isPending && !rentalsQuery.isError && completed.length === 0 && (
          <EmptyState
            imageSrc="/images/rentals/rental-hero.png"
            title="У вас нет завершенных аренд"
            description="Когда аренда закончится, информация об этом появится здесь"
          />
        )}

        {!rentalsQuery.isPending && !rentalsQuery.isError && completed.length > 0 && (
          <div className="flex flex-col gap-4 pt-2">
            {completed.map((rental) => (
              <PastRentalCard
                key={rental.id}
                rental={rental}
                onOpen={() => router.push(ROUTES.propertyRentalCompleted(propertyId, rental.id))}
              />
            ))}
          </div>
        )}
      </PageContent>
    </>
  );
}

/** Карточка списка (1302:52462): серая группа со стрелкой у края, три
 * строки условий и строка арендатора — только с арендатором. */
function PastRentalCard({
  rental,
  onOpen,
}: {
  readonly rental: Rental;
  readonly onOpen: () => void;
}): JSX.Element {
  const tenant = rental.tenant;
  return (
    <RentalGroup
      title={pastRentalCardTitle(rental)}
      onOpen={onOpen}
      openLabel={`Открыть аренду от ${rental.startDate}`}
      arrowPosition="edge"
      className="pb-4"
      contentGap="gap-4"
    >
      <TermRows rows={pastRentalRows(rental)} />
      {tenant !== null && <PastTenantRow name={rentalTenantTitle(tenant)} />}
    </RentalGroup>
  );
}

/** Строка арендатора карточки (1302:52462): белый круг с человеком, имя,
 * подзаголовок-роль «Арендатор»; внутри карточки — без собственного тапа. */
function PastTenantRow({ name }: { readonly name: string }): JSX.Element {
  return (
    <PaymentRowButton
      variant="gray"
      className="pointer-events-none px-3 py-2"
      categoryIcon={
        <CircleIcon variant="muted" aria-hidden>
          <BoldUser className="h-6 w-6 text-content" />
        </CircleIcon>
      }
      title={name}
      subtitle="Арендатор"
    />
  );
}
