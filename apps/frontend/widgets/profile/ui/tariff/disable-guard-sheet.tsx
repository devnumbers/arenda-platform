'use client';

import { type JSX } from 'react';
import { buttonVariants, Modal, ModalContent } from '@/shared/ui/design';
import { formatPaymentCountdown } from '@/widgets/profile/lib/tariff-overview';
import { usePaymentTimer } from './use-payment-timer';
import type { PendingPayment } from '@/entities/billing';

/** Гард «Нельзя отключить тариф» (#621, макет 1929-76198): при живой
 * pending-оплате кнопка «Отключить тариф» открывает шит с Primary
 * «Вернуться к оплате» (confirmUrl банка, новая вкладка — как плашка
 * #620) и отсчётом «MM:SS» от expiresAt. По истечении останавливаемся
 * на «00:00» и зовём onExpired: бэк пометит платёж failed, гард
 * перестанет открываться с обновлением подписки. */
export function DisableGuardSheet({
  pending,
  open,
  onOpenChange,
  onExpired,
}: {
  readonly pending: PendingPayment;
  readonly open: boolean;
  readonly onOpenChange: (open: boolean) => void;
  readonly onExpired: () => void;
}): JSX.Element {
  const now = usePaymentTimer(pending.expiresAt, open, onExpired);

  return (
    <Modal open={open} onOpenChange={onOpenChange}>
      <ModalContent
        title="Нельзя отключить тариф"
        description="Пока есть незавершенная оплата, нельзя отключить тариф. Вернитесь к оплате или дождитесь окончания времени на оплату"
      >
        <div className="flex flex-col items-center gap-6">
          {/* Ссылка, а не Button: confirmUrl ведёт на форму банка в новой
              вкладке; белый цвет — обёрткой (#620, легаси-сброс
              `a { color: inherit }` вне @layer). */}
          <div className="w-full text-white">
            <a
              href={pending.confirmUrl}
              target="_blank"
              rel="noopener noreferrer"
              className={buttonVariants({ variant: 'primary', className: 'w-full' })}
            >
              Вернуться к оплате
            </a>
          </div>

          <p className="m-0 flex items-center gap-2 text-sm font-medium leading-4 text-primary">
            Время на оплату
            <span className="font-mono">{formatPaymentCountdown(pending.expiresAt, now)}</span>
          </p>
        </div>
      </ModalContent>
    </Modal>
  );
}
