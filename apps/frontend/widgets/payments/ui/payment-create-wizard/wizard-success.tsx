'use client';

import type { JSX } from 'react';
import type { Payment } from '@/entities/payment';
import {
  CategoryIcon,
  categoryStyle,
  type CategoryStyle,
} from '@/features/payment-categories';
import { Button, StatusIcon } from '@/shared/ui/design';
import { successScreenCopy } from '@/features/payments';
import type { PaymentDraftType } from '@/features/payments';
import { firstOccurrencePreview } from '../../lib/first-occurrence';
import { WizardBottomBar } from './wizard-chrome';

/**
 * Экран успеха визарда (#464, Figma 835:19893): иконка категории с бейджем
 * «выполнено», заголовок «Вы создали платеж/автоплатеж «Название»» и дата
 * первого вхождения из серверного ответа. Кнопка «Посмотреть платеж»
 * скрыта до реализации страницы платежа (решение #449); закрытие возвращает
 * на источник истории.
 */

export type WizardSuccessProps = {
  readonly created: Payment;
  readonly draftType: PaymentDraftType;
  readonly onClose: () => void;
};

export function WizardSuccess({
  created,
  draftType,
  onClose,
}: WizardSuccessProps): JSX.Element {
  const style: CategoryStyle = categoryStyle('default', created.category.slug);
  const copy = successScreenCopy({
    draftType,
    title: created.title,
    amountKopecks: created.amountKopecks,
    recurrence: created.recurrence,
    firstOccurrence: firstOccurrencePreview(
      created.recurrence,
      created.since,
      created.endDate,
    ),
  });

  return (
    <div className="flex flex-col gap-8 px-6 pt-16">
      <div className="flex flex-col items-center gap-8">
        <span className="relative">
          <CategoryIcon icon={style.icon} color={style.color} className="h-24 w-24" />
          <StatusIcon
            status="good"
            className="absolute right-0 bottom-0 h-12 w-12 translate-x-1/4 translate-y-1/4"
          />
        </span>
        <div className="flex flex-col gap-3 self-stretch">
          <h1 className="text-center font-sans text-xl leading-6 font-semibold whitespace-pre-line text-content">
            {copy.heading}
          </h1>
          <p className="text-center text-base leading-[18px] whitespace-pre-line text-content-secondary">
            {copy.description}
          </p>
        </div>
      </div>
      <WizardBottomBar>
        <Button className="w-full" onClick={onClose}>
          Хорошо, закрыть
        </Button>
      </WizardBottomBar>
    </div>
  );
}
