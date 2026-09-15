'use client';

import type { JSX } from 'react';
import { buttonVariants } from '@/shared/ui/design';
import { formatPaymentCountdown } from '@/widgets/profile/lib/tariff-overview';
import { usePaymentTimer } from './use-payment-timer';
import type { PendingPayment } from '@/entities/billing';

/** Ссылка «Вернуться к оплате» на confirmUrl банка. Ссылка, а не Button:
 * форма открывается в новой вкладке. Белый цвет — обёрткой:
 * легаси-сброс `a { color: inherit }` вне @layer бьёт цветовые утилиты
 * на самой ссылке (#620). */
export function PendingPaymentLink({
  confirmUrl,
}: {
  readonly confirmUrl: string;
}): JSX.Element {
  return (
    <div className="w-full text-white">
      <a
        href={confirmUrl}
        target="_blank"
        rel="noopener noreferrer"
        className={buttonVariants({ variant: 'primary', className: 'w-full' })}
      >
        Вернуться к оплате
      </a>
    </div>
  );
}

/** Кнопка «Вернуться к оплате» и отсчёт «Время на оплату MM:SS» живой
 * pending-оплаты (#616) — общий блок плашки «Тарифа» (#620), футера
 * «Выбрать тариф» (#623) и гарда отключения (#621). Таймер тикает, пока
 * `enabled`; по истечении останавливается на «00:00» и зовёт onExpired —
 * реакция экранная (инвалидация подписки, закрытие гарда). */
export function PendingPaymentCta({
  pending,
  enabled = true,
  onExpired,
  withCountdown = true,
}: {
  readonly pending: PendingPayment;
  readonly enabled?: boolean;
  readonly onExpired?: () => void;
  readonly withCountdown?: boolean;
}): JSX.Element {
  const now = usePaymentTimer(pending.expiresAt, enabled, onExpired);

  return (
    <>
      <PendingPaymentLink confirmUrl={pending.confirmUrl} />
      {withCountdown && (
        <p className="m-0 flex items-center gap-2 text-sm font-medium leading-4 text-primary">
          Время на оплату
          <span className="font-mono">{formatPaymentCountdown(pending.expiresAt, now)}</span>
        </p>
      )}
    </>
  );
}
