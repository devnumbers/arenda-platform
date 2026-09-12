'use client';

import { type JSX, useEffect, useState } from 'react';
import { buttonVariants } from '@/shared/ui/design';
import {
  formatPaymentCountdown,
  pendingPaymentDescription,
} from '@/widgets/profile/lib/tariff-overview';
import type { PendingPayment } from '@/entities/billing';

/** Синяя плашка «Ожидаем оплату» главного экрана «Тариф» (#620, макет
 * 1927-75410): фон rgba(43,127,255,0.1), скругление 32, паддинг 32,
 * Primary-кнопка «Вернуться к оплате» на confirmUrl банка и отсчёт
 * «MM:SS» от expiresAt живой pending-оплаты (#616). Правило владельца:
 * пока платёж жив, плашка не пропадает и не меняется при переключениях —
 * данные приходят из кэша GET /subscription и переживают уход/возврат
 * на экран. По истечении срока останавливаемся на «00:00» и дергаем
 * refetch: бэк помечает платёж failed, плашка исчезает с обновлением
 * подписки. */
export function PendingPaymentPlaque({
  pending,
  onExpired,
}: {
  readonly pending: PendingPayment;
  readonly onExpired: () => void;
}): JSX.Element {
  const [now, setNow] = useState(() => new Date());

  useEffect(() => {
    const expiresMs = new Date(pending.expiresAt).getTime();
    const timer = setInterval(() => {
      const current = new Date();
      setNow(current);
      if (current.getTime() >= expiresMs) {
        clearInterval(timer);
        onExpired();
      }
    }, 1000);
    return () => clearInterval(timer);
  }, [pending.expiresAt, onExpired]);

  return (
    <div className="flex flex-col gap-6 rounded-[32px] bg-[rgba(43,127,255,0.1)] p-8 text-primary">
      <div className="flex flex-col gap-2">
        <h2 className="m-0 text-2xl font-semibold leading-8">Ожидаем оплату</h2>
        <p className="m-0 text-base leading-[18px]">{pendingPaymentDescription(pending)}</p>
      </div>

      {/* Ссылка, а не Button: confirmUrl ведёт на форму банка в новой
          вкладке. Белый цвет — обёрткой: легаси-сброс `a { color: inherit }`
          вне @layer бьёт цветовые утилиты на самой ссылке (#620). */}
      <div className="text-white">
        <a
          href={pending.confirmUrl}
          target="_blank"
          rel="noopener noreferrer"
          className={buttonVariants({ variant: 'primary', className: 'w-full' })}
        >
          Вернуться к оплате
        </a>
      </div>

      <p className="m-0 flex items-center gap-2 text-sm font-medium leading-4">
        Время на оплату
        <span className="font-mono">{formatPaymentCountdown(pending.expiresAt, now)}</span>
      </p>
    </div>
  );
}
