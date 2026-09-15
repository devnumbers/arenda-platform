'use client';

import type { JSX } from 'react';
import { pendingPaymentDescription } from '@/widgets/profile/lib/tariff-overview';
import { PendingPaymentCta } from './pending-payment-cta';
import type { PendingPayment } from '@/entities/billing';

/** Синяя плашка «Ожидаем оплату» главного экрана «Тариф» (#620, макет
 * 1927-75410): фон rgba(43,127,255,0.1), скругление 32, паддинг 32,
 * Primary-кнопка «Вернуться к оплате» на confirmUrl банка и отсчёт
 * «MM:SS» от expiresAt живой pending-оплаты (#616) — общий CTA
 * (PendingPaymentCta). Правило владельца: пока платёж жив, плашка не
 * пропадает и не меняется при переключениях — данные приходят из кэша
 * GET /subscription и переживают уход/возврат на экран. По истечении
 * срока останавливаемся на «00:00» и дергаем refetch: бэк помечает
 * платёж failed, плашка исчезает с обновлением подписки. */
export function PendingPaymentPlaque({
  pending,
  onExpired,
}: {
  readonly pending: PendingPayment;
  readonly onExpired: () => void;
}): JSX.Element {
  return (
    <div className="flex flex-col gap-6 rounded-[32px] bg-[rgba(43,127,255,0.1)] p-8 text-primary">
      <div className="flex flex-col gap-2">
        <h2 className="m-0 text-2xl font-semibold leading-8">Ожидаем оплату</h2>
        <p className="m-0 text-base leading-[18px]">{pendingPaymentDescription(pending)}</p>
      </div>

      <PendingPaymentCta pending={pending} onExpired={onExpired} />
    </div>
  );
}
