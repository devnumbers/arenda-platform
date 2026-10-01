'use client';

import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import type { Payment } from '@/entities/payment';
import {
  CategoryIcon,
  categoryStyle,
  type CategoryStyle,
} from '@/features/payment-categories';
import { Button, StatusIcon, StickyBottomBar } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import { successScreenCopy } from '@/features/payments';
import type { PaymentDraftType } from '@/features/payments';
import { firstOccurrencePreview } from '../../lib/first-occurrence';
import { WizardBottomBar } from './wizard-chrome';

/**
 * Экран успеха визарда (Figma 835:19893; структура — канон успеха операций
 * 1858:105544, решение владельца 01.10): иконка категории кружком 96
 * (глиф ~52 — те же пропорции, что 24 в кружке 44) с бейджем «выполнено»
 * ~52 в углу, заголовок «Вы создали платеж/автоплатеж «Название»» —
 * название только если его ввели (правка владельца 2026-08-31), большая
 * сумма со знаком (расход с минусом, доход с плюсом) и строка расписания
 * из серверного ответа. Кнопки: «Хорошо, закрыть» (история назад) и
 * «Посмотреть платеж» (страница платежа; скрыта была до её реализации,
 * #449).
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
    recurrence: created.recurrence,
    firstOccurrence: firstOccurrencePreview(
      created.recurrence,
      created.since,
      created.endDate,
    ),
  });
  // Знак — отображение направления (канон операций: «-1 000 ₽»/«+1 000 ₽»);
  // сумма хранится положительной. Формат — только канонический форматтер.
  const signedAmount =
    created.type === 'expense'
      ? `-${formatMoneyKopecks(created.amountKopecks)}`
      : `+${formatMoneyKopecks(created.amountKopecks)}`;

  return (
    <div className="flex flex-col items-center px-8 pt-16">
      <div className="flex flex-col items-center gap-8">
        {/* Композиция фрейма 1858:105549 (канон успеха операций): кружок
            категории 96 (глиф ~52), бейдж ~52 накладывается в угол
            (61, 61). */}
        <span className="relative block h-24 w-24">
          <CategoryIcon
            icon={style.icon}
            color={style.color}
            className="h-24 w-24 [&>svg]:h-13 [&>svg]:w-13"
          />
          <StatusIcon
            status="good"
            className="absolute top-[61px] left-[61px] h-13 w-13"
          />
        </span>
        <div className="flex flex-col gap-1 self-stretch">
          <h1 className="text-center text-xl leading-6 font-normal whitespace-pre-line break-words text-content">
            {copy.heading}
          </h1>
          {copy.description !== undefined && (
            <p className="text-center text-base leading-[18px] text-content-secondary">
              {copy.description}
            </p>
          )}
        </div>
        <p className="text-center text-[2.5rem] leading-11 font-semibold break-words text-content">
          {signedAmount}
        </p>
      </div>
      {/* Канон панелей шага: StickyBottomBar прижимает кнопки к низу
          (колонка 560 на ПК, во всю ширину на планшете и уже) — как на
          всех шагах визарда. */}
      <StickyBottomBar>
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
      </StickyBottomBar>
    </div>
  );
}
