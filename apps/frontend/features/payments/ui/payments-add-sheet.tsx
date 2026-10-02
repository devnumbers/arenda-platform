'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import Image from 'next/image';
import { useRouter } from 'next/navigation';
import { Add, Edit } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import {
  latestPaymentDraftType,
  usePaymentWizardDraft,
  type PaymentDraftType,
} from '../lib/use-payment-wizard-draft';
import { paymentAddSheetState } from '../lib/payment-add-sheet-model';
import { Button, Modal, ModalContent } from '@/shared/ui/design';

/**
 * Единый вход в создание платежа (спека #453, история 13): клик «Добавить»
 * проверяет черновик визарда — на какой бы поверхности ни жил вход (#1066:
 * страница платежей объекта, страница объекта, каталог секций, глобальные
 * «Объекты»). Есть черновик — Figma 1134:40451: заголовок «У вас есть
 * черновик» и две кнопки — «Продолжить черновик» (визард того же типа
 * восстанавливается на сохранённом шаге) и «Создать новый» (черновики
 * объекта стираются, Figma 837:21376), после которой в той же модалке
 * показывается выбор типа — ровно как без черновика (Figma 847:11688).
 * Черновика нет — выбор типа сразу.
 *
 * Режимы: без `fixedType` — шит сам разбирает черновики обоих типов
 * (состояние — paymentAddSheetState); с `fixedType` (кнопки с заранее
 * известным типом: «Добавить платеж» каталога, «Добавить платёж» пустых
 * «Объектов») — модалка черновика своего типа; черновика нет — шит не
 * открывают вовсе, потребитель ведёт straight в визард (UX прежний).
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
  /** Заранее известный тип создания: модалка черновика только этого типа,
   * фазы выбора нет (её роль играет сам прямой вход при отсутствии
   * черновика — решение владельца #1066). */
  readonly fixedType?: PaymentDraftType;
};

export function PaymentsAddSheet({
  propertyId,
  open,
  onOpenChange,
  fixedType,
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
  const state = paymentAddSheetState(loaded, latest, fixedType);

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
  if (state.kind === 'draft' && phase === 'draft') {
    const draftType = state.draftType;
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
            // Оба черновика объекта стираются (Figma 837:21376); через
            // clearDraft хуков, а не storage-хелпер: тот чистит хранилище,
            // не двигая смонтированные снапшоты, и шит остался бы в
            // черновик-состоянии.
            paymentDraft.clearDraft();
            autopaymentDraft.clearDraft();
            if (fixedType !== undefined) {
              // Кнопка с известным типом: выбор-фазы нет — «Создать новый»
              // сразу ведёт в чистый визард этого типа.
              goWizard(fixedType);
              return;
            }
            setPhase('choice');
          }}
        >
          Создать новый
        </Button>
      </div>
    );
  } else if (state.kind === 'choice') {
    title = 'Выберите тип платежа';
    content = (
      <div className="grid grid-cols-2 gap-2">
        {(Object.keys(TYPE_META) as PaymentDraftType[]).map((type) => (
          <ChoiceCard key={type} type={type} onSelect={() => goWizard(type)} />
        ))}
      </div>
    );
  } else {
    // loading: контент не выбираем, чтобы выбор типа не мелькнул перед
    // черновик-состоянием.
    title = '';
    content = null;
  }

  return (
    <Modal open={open} onOpenChange={handleOpenChange}>
      {content !== null && (
        <ModalContent title={title} showClose>
          {content}
        </ModalContent>
      )}
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
