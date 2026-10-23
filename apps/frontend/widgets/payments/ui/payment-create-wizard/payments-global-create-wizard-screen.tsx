'use client';

import type { JSX } from 'react';
import {
  useEphemeralPaymentWizardDraft,
  type PaymentDraftType,
} from '@/features/payments';
import { PaymentCreateWizardFlow } from './payment-create-wizard-flow';

/**
 * Экран глобального входа визарда платежа — /payments/new (решение
 * владельца 23.10.2026, аналог /operations/new #570): тип «Платёж /
 * Автоплатёж» приходит query-параметром модалки выбора хаба, объект
 * выбирается шагом «Выбрать объект». Право правки не гейтится до рендера —
 * шаг выбора несёт только редактируемые объекты (#703), а отказ сервера
 * на сабмите показывает тост ошибки. Персистентного черновика у глобального
 * входа нет — состояние живёт в памяти смонтированного потока.
 */
export function PaymentsGlobalCreateWizardScreen({
  draftType,
}: {
  readonly draftType: PaymentDraftType;
}): JSX.Element {
  const draftController = useEphemeralPaymentWizardDraft();
  return (
    <PaymentCreateWizardFlow
      key={draftType}
      mode="global"
      draftType={draftType}
      draftController={draftController}
    />
  );
}
