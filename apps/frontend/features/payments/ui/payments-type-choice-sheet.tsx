'use client';

import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ROUTES } from '@/shared/config/routes';
import { pushPage } from '@/shared/lib/navigation';
import { Modal, ModalContent } from '@/shared/ui/design';
import type { PaymentDraftType } from '../lib/use-payment-wizard-draft';
import { PaymentTypeChoiceCards } from './payment-type-choice-cards';

/**
 * Модалка выбора типа создания на глобальном хабе «Платежи» (решение
 * владельца 23.10.2026): «+» хаба показывает те же карточки «Платёж /
 * Автоплатёж» (Figma 3065:71128), что и шит единого входа #453, но ведёт
 * в глобальный визард /payments/new — объект выбирается шагом «Выбрать
 * объект». Черновой фазы нет: черновики живут только в объектном входе.
 */
export function PaymentsTypeChoiceSheet({
  open,
  onOpenChange,
}: {
  readonly open: boolean;
  readonly onOpenChange: (open: boolean) => void;
}): JSX.Element {
  const router = useRouter();

  const goWizard = (type: PaymentDraftType): void => {
    pushPage(router, ROUTES.paymentsNew(type));
  };

  return (
    <Modal open={open} onOpenChange={onOpenChange}>
      <ModalContent title="Выберите тип платежа" showClose>
        <PaymentTypeChoiceCards onSelect={goWizard} />
      </ModalContent>
    </Modal>
  );
}
