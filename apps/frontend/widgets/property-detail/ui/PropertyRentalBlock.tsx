'use client';

import type { JSX } from 'react';
import type { Rental } from '@/entities/rental';
import { CalendarSmall } from '@/shared/assets/icons';
import { Button } from '@/shared/ui/design';
import { buildPropertyRentalBlock } from '../lib/rental-block';

type PropertyRentalBlockProps = {
  readonly rental: Rental;
  readonly onExtend: () => void;
  readonly onComplete: () => void;
};

/**
 * Заполненная секция «Аренда» на детали объекта (тикет #589; Figma
 * 1185:40820 — активная: «Оплачено 6 из 24 платежей», синяя строка дней
 * у календаря 16, прогресс-бар 6px с белой дорожкой, серый футер;
 * 1581:53905 — срок подошёл к концу: «Последний платеж оплачен», полный
 * бар, кнопки «Продлить» (белая) и «Завершить» (синяя), обе small
 * radius m, зазор пары 24 держит gap родителя). Существующие флоу:
 * продление и мастер завершения аренды.
 */
export function PropertyRentalBlock({
  rental,
  onExtend,
  onComplete,
}: PropertyRentalBlockProps): JSX.Element {
  const block = buildPropertyRentalBlock(rental);

  return (
    <div className="flex flex-col px-6 pb-6 pt-4" data-testid="property-rental-block">
      <div className="flex flex-col gap-4">
        <span className="text-base font-medium leading-[18px] text-content">
          {block.paidTitle}
        </span>
        <div className="flex flex-col gap-3">
          {block.paymentLine !== null && (
            <div className="flex items-center gap-1.5">
              <CalendarSmall className="h-4 w-4 shrink-0 text-primary" aria-hidden />
              <span className="text-sm font-medium leading-4 text-primary">
                {block.paymentLine}
              </span>
            </div>
          )}
          {block.percent !== null && (
            <div
              role="progressbar"
              aria-valuemin={0}
              aria-valuemax={100}
              aria-valuenow={block.percent}
              aria-label={block.paidTitle}
              className="h-1.5 w-full overflow-hidden rounded-pill bg-surface"
            >
              <div
                className="h-full rounded-pill bg-primary"
                style={{ width: `${block.percent}%`, minWidth: block.percent > 0 ? 6 : 0 }}
              />
            </div>
          )}
        </div>
        <span className="text-sm leading-4 text-content-secondary">{block.footerLine}</span>
      </div>
      {block.endOfTerm && (
        <div className="mt-6 flex gap-2">
          <Button
            variant="white"
            size="small"
            radius="m"
            className="flex-1"
            onClick={onExtend}
          >
            Продлить
          </Button>
          <Button
            variant="primary"
            size="small"
            radius="m"
            className="flex-1"
            onClick={onComplete}
          >
            Завершить
          </Button>
        </div>
      )}
    </div>
  );
}
