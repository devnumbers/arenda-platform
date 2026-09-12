'use client';

import { type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { useInfiniteScroll } from '@/shared/lib/hooks/useInfiniteScroll';
import { clientTodayIso } from '@/entities/payment';
import {
  usePayments,
  usePropertyOperationsPaged,
  usePropertyOverdueOperations,
} from '@/features/payments';
import { canMutateProperty, useProperty } from '@/features/properties';
import {
  Button,
  IconButton,
  PageContent,
  StickyBottomBar,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import { overduePaymentIdsOf, sortPaymentsByNextOccurrence } from '@/features/payments';
import {
  OverdueOperationRow,
  PaymentRow,
  PaymentsEmptyState,
  PaymentsSkeleton,
  PaymentsStateCard,
} from './payments-sections';

/**
 * Страницы секций «Платежей объекта» (Figma 1043:60174/1043:60502, решение
 * владельца): клик на заголовок секции главного экрана ведёт сюда, где виден
 * весь список. Одна форма на три среза: просроченные операции объекта
 * (строки с red-стилизацией, порции по 50 со скроллом), все платежи и
 * автоплатежи (правила в порядке ближайшего вхождения; паузные показываются
 * — «там уже всё видно», в отличие от главного экрана). Пустое состояние —
 * иллюстрация и пояснение по фреймам; кнопка «Добавить …» ведёт прямо в
 * визард нужного типа и скрыта у смотрящего (история 47) и на архиве (#446).
 */

export type PaymentsCatalogVariant = 'overdue' | 'payments' | 'auto';

const CATALOG: Record<
  PaymentsCatalogVariant,
  { title: string; emptyTitle: string; emptyHint: string }
> = {
  overdue: {
    title: 'Просроченные операции',
    emptyTitle: 'Нет просроченных операций',
    emptyHint: 'Когда платеж просрочится, он будет здесь',
  },
  payments: {
    title: 'Платежи объекта',
    emptyTitle: 'Нет платежей',
    emptyHint: 'Напомним, когда нужно будет отметить оплату, вы вручную отметите платеж',
  },
  auto: {
    title: 'Автоплатежи объекта',
    emptyTitle: 'Нет автоплатежей',
    emptyHint: 'Предупредим о платеже, потом автоматически отметим оплату',
  },
};

/** Иллюстрация пустого состояния; у просроченных своей нет — берётся
 * платежная (решение владельца). */
const EMPTY_IMAGE: Record<PaymentsCatalogVariant, string> = {
  overdue: '/images/payments/empty-payments.png',
  payments: '/images/payments/empty-payments.png',
  auto: '/images/payments/empty-autopayments.png',
};

export function PaymentsCatalogScreen({
  propertyId,
  variant,
}: {
  readonly propertyId: string;
  readonly variant: PaymentsCatalogVariant;
}): JSX.Element {
  const router = useRouter();
  const meta = CATALOG[variant];

  const propertyQuery = useProperty(propertyId);
  const property = propertyQuery.isSuccess ? propertyQuery.data : undefined;
  const canMutate = canMutateProperty(property);

  return (
    <>
      <TopNav
        leading={
          <IconButton
            icon={<ArrowLeft />}
            label="Назад"
            onClick={() => goBack(router, ROUTES.propertyPayments(propertyId))}
          />
        }
      >
        <TopNavTitle title={meta.title} />
      </TopNav>

      <PageContent>
        {variant === 'overdue' ? (
          <OverdueList propertyId={propertyId} meta={meta} />
        ) : (
          <RulesList propertyId={propertyId} variant={variant} meta={meta} />
        )}
      </PageContent>

      {canMutate && variant !== 'overdue' && (
        <StickyAddButton
          label={variant === 'payments' ? 'Добавить платеж' : 'Добавить автоплатеж'}
          onClick={() =>
            router.push(ROUTES.propertyPaymentNew(propertyId, variant === 'payments' ? 'payment' : 'autopayment'))
          }
        />
      )}
    </>
  );
}

/** Полный список просроченных операций объекта: asc — долг разбирают по
 * порядку накопления, порции по 50 с бесконечным скроллом; строка ведёт
 * на страницу операции. */
function OverdueList({
  propertyId,
  meta,
}: {
  readonly propertyId: string;
  readonly meta: { emptyTitle: string; emptyHint: string };
}): JSX.Element {
  const router = useRouter();
  const overdueQuery = usePropertyOperationsPaged(propertyId, {
    status: 'overdue',
    order: 'asc',
  });
  const sentinelRef = useInfiniteScroll(
    () => {
      if (overdueQuery.hasNextPage && !overdueQuery.isFetchingNextPage) {
        void overdueQuery.fetchNextPage();
      }
    },
    overdueQuery.hasNextPage === true,
  );

  const today = clientTodayIso();
  const operations = overdueQuery.data ?? [];

  if (overdueQuery.isPending) {
    return (
      <div className="flex flex-col gap-2">
        <PaymentsSkeleton withHeading />
        <PaymentsSkeleton />
      </div>
    );
  }
  if (overdueQuery.isError) {
    return (
      <PaymentsStateCard
        title="Не удалось загрузить просроченные"
        hint="Проверьте подключение и попробуйте снова"
        action={
          <Button variant="secondary" size="small" onClick={() => void overdueQuery.refetch()}>
            Повторить
          </Button>
        }
      />
    );
  }
  if (operations.length === 0) {
    return <CatalogEmpty meta={meta} variantForImage="overdue" />;
  }
  return (
    <div className="flex flex-col">
      <section className="flex flex-col">
        {operations.map((operation) => (
          <OverdueOperationRow
            key={operation.id}
            operation={operation}
            today={today}
            variant="white"
            className="py-3"
            onSelect={() => router.push(ROUTES.propertyOperation(propertyId, operation.id))}
          />
        ))}
      </section>
      {overdueQuery.hasNextPage === true && <div ref={sentinelRef} aria-hidden />}
    </div>
  );
}

/** Полный список правил среза: сортировка по ближайшему вхождению, паузные
 * показываются с подписью «На паузе» (в конце списка). */
function RulesList({
  propertyId,
  variant,
  meta,
}: {
  readonly propertyId: string;
  readonly variant: PaymentsCatalogVariant;
  readonly meta: { emptyTitle: string; emptyHint: string };
}): JSX.Element {
  const router = useRouter();
  const paymentsQuery = usePayments(propertyId);
  // Точки просрочки на плашках (1323:61133, State=Expired): первая порция
  // просроченных операций объекта (порция 50 — глубже не бейджим).
  const overdueQuery = usePropertyOverdueOperations(propertyId);
  const overduePaymentIds = overduePaymentIdsOf(overdueQuery.data ?? []);
  const today = clientTodayIso();

  if (paymentsQuery.isPending) {
    return (
      <div className="flex flex-col gap-2">
        <PaymentsSkeleton withHeading />
        <PaymentsSkeleton />
      </div>
    );
  }
  if (paymentsQuery.isError) {
    return (
      <PaymentsStateCard
        title="Не удалось загрузить платежи"
        hint="Проверьте подключение и попробуйте снова"
        action={
          <Button variant="secondary" size="small" onClick={() => void paymentsQuery.refetch()}>
            Повторить
          </Button>
        }
      />
    );
  }

  const rules = paymentsQuery.data
    .filter((payment) => (variant === 'auto' ? payment.autoPay : true));
  const sorted = sortPaymentsByNextOccurrence(rules, today);

  if (sorted.length === 0) {
    return <CatalogEmpty meta={meta} variantForImage={variant} />;
  }
  return (
    <section className="flex flex-col">
      {sorted.map((payment) => (
        <PaymentRow
          key={payment.id}
          payment={payment}
          today={today}
          variant="white"
          hasOverdue={overduePaymentIds.has(payment.id)}
          onSelect={() => router.push(ROUTES.propertyPayment(propertyId, payment.id))}
        />
      ))}
    </section>
  );
}

/** Пустое состояние страницы: иллюстрация, заголовок и пояснение — по
 * фреймам 1043:60174/1043:60502 (общий PaymentsEmptyState). */
function CatalogEmpty({
  meta,
  variantForImage,
}: {
  readonly meta: { emptyTitle: string; emptyHint: string };
  readonly variantForImage: PaymentsCatalogVariant;
}): JSX.Element {
  return (
    <PaymentsEmptyState
      image={EMPTY_IMAGE[variantForImage]}
      title={meta.emptyTitle}
      hint={meta.emptyHint}
    />
  );
}

/** Закреплённая кнопка добавления — вход сразу в визард нужного типа. */
function StickyAddButton({
  label,
  onClick,
}: {
  readonly label: string;
  readonly onClick: () => void;
}): JSX.Element {
  return (
    <StickyBottomBar>
      <Button className="w-full" onClick={onClick}>
        {label}
      </Button>
    </StickyBottomBar>
  );
}
