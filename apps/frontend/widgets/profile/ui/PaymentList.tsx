'use client';

import type { JSX } from 'react';
import Image from 'next/image';
import { useRouter } from 'next/navigation';
import { Button, Skeleton } from '@/shared/ui/design';
import { useSubscriptionPayments } from '@/features/billing';
import { PaymentRowButton } from '@/entities/payment';
import { getTariffLabel } from '@/entities/user';
import { Undo } from '@/shared/assets/icons';
import { dateToIsoLocal } from '@/shared/lib/calendar';
import { ROUTES } from '@/shared/config/routes';
import {
  groupSubscriptionPaymentsByDate,
  paymentCardMask,
  paymentRowAmountProps,
  paymentRowSubtitle,
} from '../lib/payment-history-model';

/** Аватар строки — иконка тарифа на голубом круге с белым кантом
 * (Figma 1877-68603, Category Icon 44); у возврата — канон Undo. */
function TariffAvatar({ refunded }: { readonly refunded: boolean }): JSX.Element {
  return (
    <span
      aria-hidden
      className="flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-primary/10 shadow-[0_0_0_2.5px_var(--dl-surface)]"
    >
      {refunded ? (
        <Undo className="h-6 w-6 text-content" />
      ) : (
        <Image src="/images/tariff/tariff-avatar.png" alt="" width={28} height={28} />
      )}
    </span>
  );
}

function HistorySkeleton(): JSX.Element {
  return (
    <div className="flex flex-col gap-6" role="status" aria-label="Загружаем историю операций">
      {[0, 1].map((group) => (
        <section key={group} className="flex flex-col">
          <Skeleton className="mx-6 mt-6 h-6 w-32" />
          {[0, 1].map((row) => (
            <div key={row} className="flex items-center gap-3 px-6 py-3">
              <Skeleton className="h-11 w-11 rounded-full" />
              <Skeleton className="h-4 w-28" />
              <span className="flex-1" />
              <Skeleton className="h-4 w-20" />
            </div>
          ))}
        </section>
      ))}
    </div>
  );
}

/** Экран «Операции» — история оплат подписки (#624, Figma 1877-68603):
 * группировка по датам (канон дат shared/lib), строки Row Button с
 * аватаром тарифа, знаковой суммой и маской карты. Название экрана —
 * копирайт-решение владельца; домен — Subscription Payments, с учётными
 * «Операциями» (доходы/расходы) не смешивается. Пусто — центрированный
 * серый текст без иллюстрации (макет 1886-109856 проще канона EmptyState
 * с PNG — отклонение зафиксировано в тикете). */
export function PaymentList(): JSX.Element {
  const { data: payments, isPending, isError, refetch } = useSubscriptionPayments();
  const router = useRouter();

  if (isError) {
    return (
      <div className="flex flex-col items-center gap-4 py-16 text-center">
        <p className="m-0 text-base leading-[18px] text-content-secondary">
          Не удалось загрузить историю операций
        </p>
        <Button onClick={() => void refetch()} variant="secondary">
          Повторить
        </Button>
      </div>
    );
  }

  if (isPending) {
    return <HistorySkeleton />;
  }

  if (payments.length === 0) {
    return (
      <div className="flex min-h-[60vh] items-center justify-center">
        <p className="m-0 text-base leading-[18px] text-content-secondary">Нет операций</p>
      </div>
    );
  }

  const groups = groupSubscriptionPaymentsByDate(payments, dateToIsoLocal(new Date()));

  return (
    <div className="flex flex-col gap-6 pb-6" data-testid="payment-history-list">
      {groups.map((group) => (
        <section key={group.date} className="flex flex-col">
          <h2 className="mx-0 mt-0 mb-2 px-6 pt-6 text-xl font-semibold leading-6 text-content">
            {group.label}
          </h2>
          {group.payments.map((payment) => {
            const amount = paymentRowAmountProps(payment);
            return (
              <PaymentRowButton
                key={payment.id}
                className="py-3"
                categoryIcon={<TariffAvatar refunded={payment.status === 'refunded'} />}
                title={`Тариф ${getTariffLabel(payment.tariff.name)}`}
                subtitle={paymentRowSubtitle(payment)}
                description={paymentCardMask(payment)}
                amountKopecks={amount.amountKopecks}
                signedAmount={amount.signedAmount}
                valueClassName={amount.valueClassName}
                onSelect={() => router.push(ROUTES.profilePaymentDetail(payment.id))}
              />
            );
          })}
        </section>
      ))}
    </div>
  );
}
