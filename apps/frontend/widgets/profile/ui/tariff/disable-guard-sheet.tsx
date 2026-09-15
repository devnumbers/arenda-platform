'use client';

import { type JSX } from 'react';
import { Modal, ModalContent } from '@/shared/ui/design';
import { PendingPaymentCta } from './pending-payment-cta';
import type { PendingPayment } from '@/entities/billing';

/** Гард «Нельзя отключить тариф» (#621, макет 1929-76198): при живой
 * pending-оплате кнопка «Отключить тариф» открывает шит с Primary
 * «Вернуться к оплате» (confirmUrl банка, новая вкладка — как плашка
 * #620) и отсчётом «MM:SS» от expiresAt — общий CTA (PendingPaymentCta).
 * По истечении останавливаемся на «00:00» и зовём onExpired: бэк пометит
 * платёж failed, гард перестанет открываться с обновлением подписки. */
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
  return (
    <Modal open={open} onOpenChange={onOpenChange}>
      <ModalContent
        title="Нельзя отключить тариф"
        description="Пока есть незавершенная оплата, нельзя отключить тариф. Вернитесь к оплате или дождитесь окончания времени на оплату"
      >
        <div className="flex flex-col items-center gap-6">
          <PendingPaymentCta pending={pending} enabled={open} onExpired={onExpired} />
        </div>
      </ModalContent>
    </Modal>
  );
}
