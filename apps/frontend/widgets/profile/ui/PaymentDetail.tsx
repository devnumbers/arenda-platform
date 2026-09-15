'use client';

import type { JSX } from 'react';
import Image from 'next/image';
import { useRouter } from 'next/navigation';
import clsx from 'clsx';
import {
  Button,
  Skeleton,
  StickyBottomBar,
  SubScreenShell,
} from '@/shared/ui/design';
import {
  useSubscriptionPayment,
} from '@/features/billing';
import type { SubscriptionPayment } from '@/entities/billing';
import { PAYMENT_STATUS_LABELS } from '@/entities/billing';
import { getTariffLabel } from '@/entities/user';
import { goBack } from '@/shared/lib/navigation';
import { usePaymentTimer } from './tariff/use-payment-timer';
import { PendingPaymentLink } from './tariff/pending-payment-cta';
import { cn } from '@/shared/lib/cn';
import { formatDateTimeHeading } from '@/shared/lib/date-format';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import { ROUTES } from '@/shared/config/routes';
import {
  isPaymentFormExpired,
  paymentCardMask,
  paymentFormDeadline,
  paymentPeriodLabel,
  paymentRowAmountProps,
  paymentStatusTone,
} from '../lib/payment-history-model';

/** Деталь платежа (#624, Figma 1883-71611 / 1904-40495 / 1892-111236 /
 * 1883-72212): в шапке — дата-время платежа, герой — аватар тарифа 96,
 * название и сумма 40pt (знак и цвет по статусу), блок «Подробнее» —
 * способ оплаты / период / статус. У живого pending с формой банка —
 * «Вернуться к оплате» и поллинг статуса (useSubscriptionPayment); после
 * серверного дедлайна формы (expiresAt, #680) ссылку прячет.
 * Shell собирается внутри: заголовок шапки зависит от данных. */
export function PaymentDetail({ id }: PaymentDetailProps): JSX.Element {
  const { data: payment, isPending, isError, refetch } = useSubscriptionPayment(id);

  return (
    <SubScreenShell
      title={payment === undefined ? 'Платёж' : formatDateTimeHeading(payment.createdAt)}
      fallbackHref={ROUTES.profilePayments}
    >
      {isError && (
        <div className="flex flex-col items-center gap-4 py-16 text-center">
          <p className="m-0 text-base leading-[18px] text-content-secondary">
            Не удалось загрузить платёж
          </p>
          <Button onClick={() => void refetch()} variant="secondary">
            Повторить
          </Button>
        </div>
      )}
      {isPending && <PaymentDetailSkeleton />}
      {!isError && !isPending && payment === undefined && <PaymentNotFound />}
      {!isError && !isPending && payment !== undefined && (
        <PaymentDetailBody payment={payment} />
      )}
    </SubScreenShell>
  );
}

export type PaymentDetailProps = {
  readonly id: string;
};

function PaymentDetailSkeleton(): JSX.Element {
  return (
    <div className="flex flex-col gap-6" role="status" aria-label="Загружаем платёж">
      <div className="flex flex-col items-center gap-6 py-6">
        <Skeleton className="h-24 w-24 rounded-full" />
        <Skeleton className="h-6 w-44" />
        <Skeleton className="h-11 w-40" />
      </div>
      <div className="flex flex-col gap-2 px-6">
        {[0, 1, 2].map((row) => (
          <div key={row} className="flex gap-3">
            <Skeleton className="h-4 w-28" />
            <Skeleton className="h-4 w-32" />
          </div>
        ))}
      </div>
    </div>
  );
}

function PaymentNotFound(): JSX.Element {
  const router = useRouter();
  return (
    <div className="flex flex-col items-center gap-4 py-16 text-center">
      <p className="m-0 text-base leading-[18px] text-content-secondary">Платёж не найден</p>
      <Button onClick={() => goBack(router, ROUTES.profilePayments)} variant="secondary">
        Назад
      </Button>
    </div>
  );
}

function DetailRow({
  label,
  value,
  valueClassName,
}: {
  readonly label: string;
  readonly value: string;
  readonly valueClassName?: string;
}): JSX.Element {
  return (
    <div className="flex gap-3">
      <span className="m-0 flex-1 text-sm leading-4 text-content-secondary">{label}</span>
      <span className={cn('m-0 flex-1 text-sm leading-4 text-content', valueClassName)}>
        {value}
      </span>
    </div>
  );
}

function PaymentDetailBody({ payment }: { readonly payment: SubscriptionPayment }): JSX.Element {
  const amount = paymentRowAmountProps(payment);
  const cardMask = paymentCardMask(payment);
  // «Вернуться к оплате» живёт у банковской pending, чья форма не истекла:
  // дедлайн — серверный expiresAt (#680, та же истина, что у провайдера).
  // Тикающий канон-хук владеет временем — рендер остаётся чистым
  // (react-hooks/purity), после дедлайна ссылка исчезает сама.
  const resumeDeadline = paymentFormDeadline(payment);
  const now = usePaymentTimer(resumeDeadline ?? '', resumeDeadline !== null);
  const resumeUrl =
    resumeDeadline !== null && !isPaymentFormExpired(payment, now)
      ? payment.paymentUrl
      : null;

  return (
    <>
      <div className="flex flex-col gap-6 py-6">
        <div className="flex flex-col items-center gap-6">
          <Image
            src="/images/tariff/tariff-avatar.png"
            alt=""
            width={128}
            height={128}
            className="h-24 w-24 rounded-full bg-primary/10 p-[17px] shadow-[0_0_0_2.5px_var(--dl-surface)]"
          />
          <h1 className="m-0 text-xl font-normal leading-6 text-content">
            Тариф {getTariffLabel(payment.tariff.name)}
          </h1>
          <p
            className={clsx(
              'm-0 font-semibold leading-[44px] text-[40px]',
              amount.valueClassName,
            )}
          >
            {amount.signedAmount && amount.amountKopecks > 0 && '+'}
            {formatMoneyKopecks(amount.amountKopecks)}
          </p>
        </div>
        <section className="flex flex-col">
          <h2 className="mx-0 mt-0 text-xl font-semibold leading-6 text-content">
            Подробнее
          </h2>
          <div className="mt-6 flex flex-col gap-2">
            {cardMask !== undefined && (
              <DetailRow label="Способ оплаты" value={cardMask} />
            )}
            <DetailRow label="Период оплаты" value={paymentPeriodLabel(payment.period)} />
            <DetailRow
              label="Статус"
              value={PAYMENT_STATUS_LABELS[payment.status]}
              valueClassName={paymentStatusTone(payment.status)}
            />
          </div>
        </section>
      </div>
      {resumeUrl !== null && (
        <StickyBottomBar>
          <PendingPaymentLink confirmUrl={resumeUrl} />
        </StickyBottomBar>
      )}
    </>
  );
}
