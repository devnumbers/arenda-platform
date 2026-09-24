'use client';

import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import { BoldKey } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import type { Rental } from '@/entities/rental';
import { rentalSuccessCopy } from '@/features/rentals';
import { Button, StatusIcon, StickyBottomBar } from '@/shared/ui/design';
import { WizardBottomBar } from './wizard-chrome';

/**
 * Экран успеха визарда аренды (Figma 1371:63753 / 1419:27092): синий круг
 * 96 с ключом и галочкой «выполнено», заголовок «Вы создали аренду» и
 * описание условий (день оплаты + сумма + способ отметки). Кнопки по
 * макету (свап #807): «Хорошо» — синяя primary закрывает поток (возврат на
 * объект), «Открыть аренду» — серая secondary ведёт на детализацию (#531).
 */

export type RentalWizardSuccessProps = {
  readonly created: Rental;
  readonly onClose: () => void;
};

export function RentalWizardSuccess({
  created,
  onClose,
}: RentalWizardSuccessProps): JSX.Element {
  const router = useRouter();
  const copy = rentalSuccessCopy({
    paymentDay: created.rentPayment.paymentDay,
    amountKopecks: created.rentPayment.amountKopecks,
    autoPay: created.rentPayment.autoPay,
  });

  return (
    <div className="flex flex-col gap-8 px-6 pt-16">
      <div className="flex flex-col items-center gap-8">
        {/* Композиция фрейма 1371:63757: круг 96, бейдж 48 накладывается
            со смещением (60, 60) — как на экране успеха платежей. */}
        <span className="relative block h-24 w-24">
          <span
            aria-hidden
            className="flex h-24 w-24 items-center justify-center rounded-pill bg-primary"
          >
            <BoldKey className="h-13 w-13 text-white" />
          </span>
          <StatusIcon status="good" className="absolute left-[60px] top-[60px] h-12 w-12" />
        </span>
        <div className="flex flex-col gap-3 self-stretch">
          <h1 className="text-center font-sans text-xl leading-6 font-semibold text-content">
            {copy.heading}
          </h1>
          <p className="text-center text-base leading-[18px] text-content-secondary">
            {copy.description}
          </p>
        </div>
      </div>
      <StickyBottomBar>
        <WizardBottomBar>
          {/* «Хорошо» закрывает поток: история назад, черновик остаётся в
              localStorage. «Открыть аренду» завершает поток созданием →
              новая страница заменяет переходную запись истории
              (CODING_STANDARDS, Navigation). */}
          <Button className="w-full" onClick={onClose}>
            Хорошо
          </Button>
          <Button
            variant="secondary"
            className="w-full"
            onClick={() => router.replace(ROUTES.propertyRental(created.propertyId))}
          >
            Открыть аренду
          </Button>
        </WizardBottomBar>
      </StickyBottomBar>
    </div>
  );
}
