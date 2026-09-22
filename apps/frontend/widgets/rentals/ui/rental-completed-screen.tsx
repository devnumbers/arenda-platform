'use client';

import { useEffect, useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import Image from 'next/image';
import { ArrowLeft, Key, TrashBin } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { formatDayMonthWithYear } from '@/shared/lib/date-format';
import type { IsoDate } from '@/shared/lib/calendar';
import {
  pastRentalCardTitle,
  pastRentalTitle,
  rentalTeaserRows,
  rentalTenantTitle,
  useDeleteRental,
  useRentals,
} from '@/features/rentals';
import { usePaymentOperationsPaged } from '@/features/payments';
import { useProperty } from '@/features/properties';
import { propertyPermissions } from '@/entities/property';
import { CategoryIcon, categoryStyle } from '@/features/payment-categories';
import type { Rental } from '@/entities/rental';
import { PaymentRowButton, type PaymentOperation } from '@/entities/payment';
import {
  Button,
  ConfirmDialog,
  EmptyState,
  IconButton,
  ListRow,
  PageContent,
  RoundActionButton,
  Skeleton,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import { RentalGroup } from './rental-group';
import { TermRows } from './term-row';
import { TenantRow } from './tenant-row';

/**
 * Завершённая детализация «Прошлых аренд» (#535, Figma 1232:61686,
 * 1550:94517): картинка-ключ и срок заголовком («24 месяца», без
 * оплаченных операций — «Не было платежей»), круглая кнопка «Посмотреть
 * итоги» — только с платежами, секции «Условия аренды» → полный экран
 * условий, «История операций» → полный список paid-вхождений платежа
 * аренды (последние три на карточке), «Арендатор», «Управление» с
 * «Удалить аренду» (только у завершённой; удаляется и Платёж, оплаченные
 * операции переживают — семантика payments). Стрелки секций — у края
 * карточки по макетам #535.
 */
export function RentalCompletedScreen({
  propertyId,
  rentalId,
}: {
  readonly propertyId: string;
  readonly rentalId: string;
}): JSX.Element {
  const router = useRouter();
  const rentalsQuery = useRentals(propertyId);
  const rental = rentalsQuery.data?.find((item) => item.id === rentalId);

  // Незавершённая аренда по прямой ссылке — её экраном остаётся /rentals.
  useEffect(() => {
    if (rentalsQuery.isSuccess && rental !== undefined && rental.status !== 'completed') {
      router.replace(ROUTES.propertyRental(propertyId));
    }
  }, [rentalsQuery.isSuccess, rental, router, propertyId]);

  return (
    <>
      <TopNav
        leading={
          <IconButton
            icon={<ArrowLeft />}
            label="Назад"
            onClick={() => goBack(router, ROUTES.propertyRentalPast(propertyId))}
          />
        }
      >
        <TopNavTitle title="Аренда" subtitle="Завершена" />
      </TopNav>

      {rentalsQuery.isPending && (
        <PageContent>
          <div className="flex flex-col gap-4 pt-6">
            <Skeleton className="h-14 w-full" />
            <Skeleton className="h-14 w-full" />
            <Skeleton className="h-14 w-full" />
          </div>
        </PageContent>
      )}

      {rentalsQuery.isError && (
        <PageContent>
          <div className="flex flex-col items-center gap-4 pt-6">
            <p className="text-center text-base leading-[18px] text-content-secondary">
              Не удалось загрузить аренду
            </p>
            <Button variant="secondary" size="small" onClick={() => void rentalsQuery.refetch()}>
              Повторить
            </Button>
          </div>
        </PageContent>
      )}

      {!rentalsQuery.isPending && !rentalsQuery.isError && rental === undefined && (
        <PageContent>
          <EmptyState
            imageSrc="/images/rentals/rental-hero.png"
            title="Аренда не найдена"
            description="Возможно, она была удалена"
          />
        </PageContent>
      )}

      {!rentalsQuery.isPending && !rentalsQuery.isError && rental?.status === 'completed' && (
        <RentalCompletedBody
          key={rental.id}
          propertyId={propertyId}
          rental={rental}
        />
      )}
    </>
  );
}

function RentalCompletedBody({
  propertyId,
  rental,
}: {
  readonly propertyId: string;
  readonly rental: Rental;
}): JSX.Element {
  const router = useRouter();
  const [deleteOpen, setDeleteOpen] = useState(false);
  const propertyQuery = useProperty(propertyId);
  const canMutate = propertyPermissions(
    propertyQuery.isSuccess ? propertyQuery.data : undefined,
  ).canEdit;
  const deleteRental = useDeleteRental(propertyId, rental.id, rental.rentPayment.paymentId);

  // История карточки — paid-вхождения платежа аренды, сначала новые
  // (серверный порядок); полной ленты здесь не нужно — первые порции.
  const operationsQuery = usePaymentOperationsPaged(propertyId, rental.rentPayment.paymentId, {
    status: 'paid',
    order: 'desc',
  });

  const operations = operationsQuery.data ?? [];
  const tenant = rental.tenant;
  const openContact =
    tenant === null
      ? undefined
      : () => router.push(ROUTES.propertyContact(propertyId, tenant.contactId));

  return (
    <PageContent>
      <div className="flex flex-col gap-12">
        <div className="flex flex-col items-center gap-4">
          <Image
            src="/images/rentals/rental-hero.png"
            alt=""
            width={96}
            height={96}
            className="h-24 w-24"
            aria-hidden
          />
          {/* Заголовок ждёт список операций: «Не было платежей» решается
              только по факту (пустой список), чтобы не мигать; при ошибке
              загрузки операций остаётся честный срок. */}
          {operationsQuery.isPending ? (
            <Skeleton className="h-8 w-56" />
          ) : (
            <h1 className="text-center text-[28px] font-semibold leading-8 text-content">
              {operationsQuery.isError
                ? pastRentalCardTitle(rental)
                : pastRentalTitle(rental, operations.length)}
            </h1>
          )}
        </div>

        {operationsQuery.isPending ? (
          <div className="flex justify-center">
            <Skeleton className="h-14 w-14 rounded-pill" />
          </div>
        ) : (
          operations.length > 0 && (
            <div className="flex justify-center">
              <RoundActionButton
                variant="secondary"
                icon={<Key />}
                caption="Посмотреть итоги"
                onClick={() =>
                  router.push(ROUTES.propertyRentalCompletedSummary(propertyId, rental.id))
                }
              />
            </div>
          )
        )}

        <div className="flex flex-col gap-4">
          <RentalGroup
            title="Условия аренды"
            contentGap="gap-4"
            arrowPosition="edge"
            onOpen={() => router.push(ROUTES.propertyRentalCompletedTerms(propertyId, rental.id))}
            openLabel="Открыть условия аренды"
          >
            <TermRows rows={rentalTeaserRows(rental)} />
          </RentalGroup>

          <RentalGroup
            title="История операций"
            arrowPosition="edge"
            className="pb-3"
            onOpen={
              operations.length > 0
                ? () => router.push(ROUTES.propertyRentalCompletedHistory(propertyId, rental.id))
                : undefined
            }
            openLabel="Открыть историю операций"
          >
            {operationsQuery.isError ? (
              <HistoryMessage text="Не удалось загрузить операции" />
            ) : operationsQuery.isPending ? (
              <div className="flex flex-col gap-4 px-6 pb-2" aria-hidden>
                <Skeleton className="h-11 w-full" />
                <Skeleton className="h-11 w-4/5" />
              </div>
            ) : operations.length > 0 ? (
              <div className="flex flex-col">
                {operations.slice(0, 3).map((operation) => (
                  <CompletedOperationRow
                    key={operation.id}
                    operation={operation}
                    today={rental.today}
                    onSelect={() =>
                      router.push(ROUTES.propertyOperation(propertyId, operation.id))
                    }
                  />
                ))}
              </div>
            ) : (
              <HistoryMessage text="Не было операций" />
            )}
          </RentalGroup>

          <RentalGroup
            title="Арендатор"
            arrowPosition="edge"
            className="pb-4"
            onOpen={openContact}
            openLabel="Открыть карточку арендатора"
          >
            {tenant !== null ? (
              <TenantRow
                tenantName={rentalTenantTitle(tenant)}
                phone={tenant.phone}
                onSelect={openContact}
              />
            ) : (
              <HistoryMessage text="Арендатор не добавлен" />
            )}
          </RentalGroup>

          {/* Удаление — только владельцу; у смотрящего секции нет. */}
          {canMutate && (
            <RentalGroup title="Управление" className="pb-3">
              <ListRow
                leading={<TrashBin className="h-6 w-6 text-danger" />}
                title={<span className="text-danger">Удалить аренду</span>}
                onSelect={() => setDeleteOpen(true)}
              />
            </RentalGroup>
          )}
        </div>
      </div>

      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title="Удалить аренду?"
        description="Аренда и её платеж будут удалены. Операции останутся в истории объекта."
        confirmLabel="Удалить"
        cancelLabel="Отмена"
        confirmVariant="danger"
        pending={deleteRental.isPending}
        onConfirm={() =>
          deleteRental.mutate(undefined, {
            onSuccess: () => {
              setDeleteOpen(false);
              goBack(router, ROUTES.propertyRentalPast(propertyId));
            },
          })
        }
      />
    </PageContent>
  );
}

/** Центрированная подпись пустой секции (1550:94517: «Не было операций»,
 * «Арендатор не добавлен» — 16/18 по центру). */
function HistoryMessage({ text }: { readonly text: string }): JSX.Element {
  return (
    <p className="px-6 py-8 text-center text-base leading-[18px] text-content-tertiary">
      {text}
    </p>
  );
}

/** Строка истории на серой карточке (1232:61686): название, дата вхождения
 * подзаголовком, сумма знаково — доход зелёным с плюсом. */
function CompletedOperationRow({
  operation,
  today,
  onSelect,
}: {
  readonly operation: PaymentOperation;
  readonly today: IsoDate;
  readonly onSelect?: () => void;
}): JSX.Element {
  const style = categoryStyle('default', operation.categorySlug);
  return (
    <PaymentRowButton
      variant="gray"
      className="px-3 py-3"
      categoryIcon={<CategoryIcon icon={style.icon} color={style.color} surface="muted" />}
      title={operation.title}
      subtitle={formatDayMonthWithYear(operation.date, today)}
      amountKopecks={
        operation.type === 'expense' ? -operation.amountKopecks : operation.amountKopecks
      }
      signedAmount
      onSelect={onSelect}
    />
  );
}
