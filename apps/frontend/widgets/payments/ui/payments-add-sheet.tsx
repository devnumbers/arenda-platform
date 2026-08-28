'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import Image from 'next/image';
import { useRouter } from 'next/navigation';
import { Add } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import {
  clearPaymentWizardDraft,
  usePaymentWizardDraft,
  type PaymentDraftType,
} from '@/features/payments';
import { Button, Modal, ModalContent } from '@/shared/ui/design';

/**
 * Шит выбора «Платёж / Автоплатёж» (спека #453, история 13): вход в создание
 * платежа с экрана объекта. Выбор типа — Figma 1134:40415 (попап) /
 * 1134:39570 (шит); состояние подтверждения при наличии черновика этого
 * типа — строка «Черновик» с «Продолжить» + кнопка «Создать новый»
 * (Figma 837:21349; черновик стирается, визард стартует с нуля). Без
 * черновика карточка ведёт в визард сразу. Индикация черновика —
 * usePaymentWizardDraft (localStorage, ключ per объект+тип); сам визард
 * и восстановление черновика — следующий срез, навигация ведёт на его
 * маршрут.
 */

/** Копирайт карточек — дословно из фреймов 1134:40415 / 1134:39570
 * (включая «Платеж» без «ё»). */
const TYPE_META: Record<
  PaymentDraftType,
  {
    readonly label: string;
    readonly description: string;
    readonly action: string;
    readonly image: string;
  }
> = {
  payment: {
    label: 'Платеж',
    description: 'Напомним, когда нужно будет отметить оплату',
    action: 'Отмечайте оплату\nвручную',
    image: '/images/payments/sheet-choice-payment.png',
  },
  autopayment: {
    label: 'Автоплатеж',
    description: 'Предупредим о платеже, потом отметим оплату',
    action: 'Отмечается автоматически',
    image: '/images/payments/sheet-choice-autopayment.png',
  },
};

export type PaymentsAddSheetProps = {
  readonly propertyId: string;
  readonly open: boolean;
  readonly onOpenChange: (open: boolean) => void;
};

export function PaymentsAddSheet({
  propertyId,
  open,
  onOpenChange,
}: PaymentsAddSheetProps): JSX.Element {
  const router = useRouter();
  const [confirmType, setConfirmType] = useState<PaymentDraftType | null>(null);
  const paymentDraft = usePaymentWizardDraft(propertyId, 'payment');
  const autopaymentDraft = usePaymentWizardDraft(propertyId, 'autopayment');
  type DraftState = ReturnType<typeof usePaymentWizardDraft>;

  const draftByType = (type: PaymentDraftType): DraftState =>
    type === 'payment' ? paymentDraft : autopaymentDraft;

  const goWizard = (type: PaymentDraftType): void => {
    router.push(ROUTES.propertyPaymentNew(propertyId, type));
  };

  const handleCardSelect = (type: PaymentDraftType): void => {
    if (draftByType(type).hasDraft) {
      setConfirmType(type);
      return;
    }
    goWizard(type);
  };

  const handleCreateNew = (type: PaymentDraftType): void => {
    clearPaymentWizardDraft(propertyId, type);
    goWizard(type);
  };

  const handleOpenChange = (nextOpen: boolean): void => {
    if (!nextOpen) {
      setConfirmType(null);
    }
    onOpenChange(nextOpen);
  };

  return (
    <Modal open={open} onOpenChange={handleOpenChange}>
      <ModalContent title="Выберите тип платежа" showClose>
        {confirmType === null ? (
          <div className="flex flex-col gap-4">
            {paymentDraft.hasDraft && (
              <DraftRow type="payment" onContinue={() => goWizard('payment')} />
            )}
            {autopaymentDraft.hasDraft && (
              <DraftRow type="autopayment" onContinue={() => goWizard('autopayment')} />
            )}
            <div className="grid grid-cols-2 gap-2">
              {(Object.keys(TYPE_META) as PaymentDraftType[]).map((type) => (
                <ChoiceCard key={type} type={type} onSelect={() => handleCardSelect(type)} />
              ))}
            </div>
          </div>
        ) : (
          <div className="flex flex-col gap-4">
            <DraftRow
              type={confirmType}
              onContinue={() => {
                goWizard(confirmType);
              }}
            />
            <Button
              className="w-full"
              leadingIcon={<Add />}
              onClick={() => {
                handleCreateNew(confirmType);
              }}
            >
              Создать новый
            </Button>
          </div>
        )}
      </ModalContent>
    </Modal>
  );
}

const draftRowClass =
  'flex w-full cursor-pointer items-center justify-between gap-3 rounded-input bg-surface-muted '
  + 'py-2.5 pl-4 pr-6 text-left outline-none transition-all focus-visible:ring-4 focus-visible:ring-primary '
  + 'hover:opacity-80 active:opacity-80';

function DraftRow({
  type,
  onContinue,
}: {
  readonly type: PaymentDraftType;
  readonly onContinue: () => void;
}): JSX.Element {
  return (
    <button
      type="button"
      onClick={onContinue}
      className={draftRowClass}
    >
      <span className="flex min-w-0 flex-col gap-0.5">
        <span className="text-[13px] leading-[15px] text-content-secondary">
          {TYPE_META[type].label}
        </span>
        <span className="text-base leading-[18px] text-content-secondary">Черновик</span>
      </span>
      <span className="text-base leading-[18px] text-content">Продолжить</span>
    </button>
  );
}

function ChoiceCard({
  type,
  onSelect,
}: {
  readonly type: PaymentDraftType;
  readonly onSelect: () => void;
}): JSX.Element {
  const meta = TYPE_META[type];
  return (
    <button
      type="button"
      onClick={onSelect}
      className="flex cursor-pointer flex-col gap-4 rounded-card bg-surface-muted p-4 text-left outline-none transition-all focus-visible:ring-4 focus-visible:ring-primary hover:opacity-80 active:opacity-80"
    >
      {/* priority — как в EmptyState: иллюстрация внутри открывающегося
          шита, ленивую загрузку наблюдатель вьюпорта может пропустить. */}
      <Image src={meta.image} alt="" width={52} height={52} priority className="h-[52px] w-[52px]" />
      <span className="flex flex-col gap-1">
        <span className="text-base font-medium text-content">{meta.label}</span>
        <span className="text-[13px] leading-[15px] text-content-secondary">{meta.description}</span>
        <span className="whitespace-pre-line text-[13px] leading-[15px] text-primary">{meta.action}</span>
      </span>
    </button>
  );
}
