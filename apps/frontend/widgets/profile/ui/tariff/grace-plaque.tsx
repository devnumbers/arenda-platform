'use client';

import type { JSX } from 'react';
import { Info } from '@/shared/assets/icons';
import { dayMonth } from '@/widgets/profile/lib/tariff-overview';

/** Красная grace-плашка главного экрана «Тариф» (#620, макет 1917-72464):
 * фон rgba(251,44,54,0.1), скругление 32, Icon/R/Info и тексты #FB2C36
 * («Оплата не прошла» H3, описание M/400 14/16). Дата дедлайна —
 * validUntil подписки ( grace v2, #615); строка «17 сентября» из макета
 * приходит параметром в готовом виде. */
export function GracePlaque({ validUntil }: { readonly validUntil?: string }): JSX.Element {
  return (
    <div className="flex gap-3 rounded-[32px] bg-[rgba(251,44,54,0.1)] p-6 pr-8 text-danger">
      <Info className="h-6 w-6 shrink-0" aria-hidden />
      <div className="flex flex-col gap-1">
        <h2 className="m-0 text-lg font-semibold leading-6">Оплата не прошла</h2>
        <p className="m-0 text-sm leading-4">
          Не хватает средств для оплаты тарифа.
          {validUntil !== undefined && ` Оплатите тариф до ${dayMonth(validUntil)}`}
        </p>
      </div>
    </div>
  );
}
