'use client';

import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import type { Payment } from '@/entities/payment';
import {
  CategoryIcon,
  categoryStyle,
  type CategoryStyle,
} from '@/features/payment-categories';
import { Button, StatusIcon } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { successScreenCopy } from '@/features/payments';
import type { PaymentDraftType } from '@/features/payments';
import { firstOccurrencePreview } from '../../lib/first-occurrence';
import { WizardBottomBar } from './wizard-chrome';

/**
 * Экран успеха визарда (Figma 835:19893): иконка выбранной категории
 * (кружок 44 из каталога, #447) с бейджем «выполнено» 48, заголовок
 * «Вы создали платеж/автоплатеж «Название»» — название только если его
 * ввели (правка владельца 2026-08-31) — и дата первого вхождения из
 * серверного ответа. Кнопки: «Хорошо, закрыть» (история назад) и
 * «Посмотреть платеж» (страница платежа; скрыта была до её реализации,
 #449).
 */

export type WizardSuccessProps = {
  readonly propertyId: string;
  readonly created: Payment;
  /** Пользователь ввёл название на шаге 2 — иначе в заголовке его нет. */
  readonly hasTypedTitle: boolean;
  readonly draftType: PaymentDraftType;
  readonly onClose: () => void;
};

export function WizardSuccess({
  propertyId,
  created,
  hasTypedTitle,
  draftType,
  onClose,
}: WizardSuccessProps): JSX.Element {
  const router = useRouter();
  const style: CategoryStyle = categoryStyle(created.category.source, created.category.slug);
  const copy = successScreenCopy({
    draftType,
    typedTitle: hasTypedTitle ? created.title : undefined,
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
        {/* Композиция фрейма 835:19989: кружок категории 44 в углу бокса
            96, бейдж 48 накладывается со смещением (60, 60). */}
        <span className="relative block h-24 w-24">
          <CategoryIcon icon={style.icon} color={style.color} />
          <StatusIcon
            status="good"
            className="absolute left-[60px] top-[60px] h-12 w-12"
          />
        </span>
        <div className="flex flex-col gap-3 self-stretch">
          <h1 className="text-center font-sans text-xl leading-6 font-semibold whitespace-pre-line text-content">
            {copy.heading}
          </h1>
          {copy.description !== undefined && (
            <p className="text-center text-base leading-[18px] whitespace-pre-line text-content-secondary">
              {copy.description}
            </p>
          )}
        </div>
      </div>
      <WizardBottomBar>
        <Button className="w-full" onClick={onClose}>
          Хорошо, закрыть
        </Button>
        <Button
          variant="secondary"
          className="w-full"
          onClick={() => router.push(ROUTES.propertyPayment(propertyId, created.id))}
        >
          Посмотреть платеж
        </Button>
      </WizardBottomBar>
    </div>
  );
}
