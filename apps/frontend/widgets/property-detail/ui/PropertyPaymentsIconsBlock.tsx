'use client';

import type { JSX } from 'react';
import { CategoryIcon, categoryStyle } from '@/features/payment-categories';
import {
  iconsFitInRow,
  stackColumnWidth,
  stackFallbackWidth,
} from '@/shared/lib/icon-row-fit';
import { useElementWidth } from '@/shared/lib/hooks/use-element-width';
import type { PropertyPaymentGroup } from '../lib/payments-strip';

/**
 * Группы круглых иконок категорий в секции «Регулярные платежи» (тикет
 * #589, Figma 1185:40820): «Автоплатежи» и «Платежи» — колонки 50/50
 * (подпись 14/16, лента кругов 44 с нахлёстом −12px и кантом 2.5px в
 * цвет карточки); красная точка 10×10 — накопленная просрочка платежа
 * (651:6759, семантика payments/CONTEXT.md). Переход — шапкой секции,
 * на страницу платежей объекта. Ряд строго в одну строку (решение
 * владельца 22.10, shared/lib/icon-row-fit): рендерится столько кругов,
 * сколько влезает в колонку — до замера ширины колонка эталонного 375.
 */
export function PropertyPaymentsIconsBlock({
  groups,
}: {
  readonly groups: ReadonlyArray<PropertyPaymentGroup>;
}): JSX.Element {
  const [stripRef, stripWidth] = useElementWidth<HTMLDivElement>();
  const single = groups.length === 1;
  const columnWidth =
    stripWidth === null ? null : stackColumnWidth(stripWidth, single);

  return (
    <div
      ref={stripRef}
      className="flex gap-4 px-6 pb-6 pt-4"
      data-testid="property-payments-icons"
    >
      {groups.map((group) => {
        const fit = iconsFitInRow(
          columnWidth ?? stackFallbackWidth(single),
          group.limit,
        );
        return (
          <div key={group.label} className="flex min-w-0 flex-1 flex-col gap-2">
            <span className="text-sm leading-4 text-content">{group.label}</span>
            <div className="flex flex-nowrap -space-x-3">
              {group.items.slice(0, fit).map((item) => {
                const style = categoryStyle(
                  item.payment.category.source,
                  item.payment.category.slug,
                );
                return (
                  <CategoryIcon
                    key={item.payment.id}
                    icon={style.icon}
                    color={style.color}
                    badge={item.overdue ? 'notification' : undefined}
                    surface="muted"
                  />
                );
              })}
            </div>
          </div>
        );
      })}
    </div>
  );
}
