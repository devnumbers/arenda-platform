'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import Image from 'next/image';
import { useRouter } from 'next/navigation';
import { Add, Edit } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import {
  clearPaymentWizardDrafts,
  latestPaymentDraftType,
  usePaymentWizardDraft,
  type PaymentDraftType,
} from '@/features/payments';
import { Button, Modal, ModalContent } from '@/shared/ui/design';

/**
 * Вход в создание платежа (спека #453, история 13): клик «Добавить»
 * проверяет черновик визарда. Есть черновик — Figma 1134:40451:
 * заголовок «У вас есть черновик» и две кнопки — «Продолжить черновик»
 * (визард того же типа восстанавливается на первый незавершённый шаг)
 * и «Создать новый» (черновики объекта стираются, Figma 837:21376),
 * после которой в той же модалке показывается выбор типа — ровно как
 * без черновика (Figma 847:11688). Черновика нет — выбор типа сразу.
 * Черновики — usePaymentWizardDraft (localStorage per объект+тип),
 * свежесть — updatedAt в payload: показывается последний тронутый.
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
  // «Создать новый» меняет контент шита на выбор типа, не закрывая
  // модалку; при закрытии шит возвращается в черновик-состояние.
  const [phase, setPhase] = useState<'draft' | 'choice'>('draft');
  const paymentDraft = usePaymentWizardDraft(propertyId, 'payment');
  const autopaymentDraft = usePaymentWizardDraft(propertyId, 'autopayment');

  // Черновики читаются из localStorage асинхронно: до загрузки контент не
  // выбираем, иначе выбор типа мелькнёт перед черновик-состоянием.
  const loaded = paymentDraft.isLoaded && autopaymentDraft.isLoaded;
  const latest: PaymentDraftType | undefined = loaded
    ? latestPaymentDraftType(paymentDraft, autopaymentDraft)
    : undefined;

  const goWizard = (type: PaymentDraftType): void => {
    router.push(ROUTES.propertyPaymentNew(propertyId, type));
  };

  const handleOpenChange = (nextOpen: boolean): void => {
    if (!nextOpen) {
      setPhase('draft');
    }
    onOpenChange(nextOpen);
  };

  let title: string;
  let content: JSX.Element | null;
  if (latest !== undefined && phase === 'draft') {
    const draftType = latest;
    title = 'У вас есть черновик';
    content = (
      <div className="flex flex-col gap-4">
        <Button
          variant="secondary"
          className="w-full"
          trailingIcon={<Edit />}
          onClick={() => goWizard(draftType)}
        >
          Продолжить черновик
        </Button>
        <Button
          className="w-full"
          trailingIcon={<Add />}
          onClick={() => {
            clearPaymentWizardDrafts(propertyId);
            setPhase('choice');
          }}
        >
          Создать новый
        </Button>
      </div>
    );
  } else {
    title = 'Выберите тип платежа';
    content = loaded ? (
      <div className="grid grid-cols-2 gap-2">
        {(Object.keys(TYPE_META) as PaymentDraftType[]).map((type) => (
          <ChoiceCard key={type} type={type} onSelect={() => goWizard(type)} />
        ))}
      </div>
    ) : null;
  }

  return (
    <Modal open={open} onOpenChange={handleOpenChange}>
      <ModalContent title={title} showClose>
        {content}
      </ModalContent>
    </Modal>
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
