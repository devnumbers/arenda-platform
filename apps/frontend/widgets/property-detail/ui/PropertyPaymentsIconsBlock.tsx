'use client';

import type { JSX } from 'react';
import { CategoryIcon, categoryStyle } from '@/features/payment-categories';
import type { PropertyPaymentGroup } from '../lib/payments-strip';

/**
 * Группы круглых иконок категорий в секции «Регулярные платежи» (тикет
 * #589, Figma 1185:40820): «Автоплатежи» и «Платежи» — колонки 50/50
 * (подпись 14/16, лента кругов 44 с нахлёстом −12px и кантом 2.5px в
 * цвет карточки); красная точка 10×10 — накопленная просрочка платежа
 * (651:6759, семантика payments/CONTEXT.md). Переход — шапкой секции,
 * на страницу платежей объекта.
 */
export function PropertyPaymentsIconsBlock({
  groups,
}: {
  readonly groups: ReadonlyArray<PropertyPaymentGroup>;
}): JSX.Element {
  return (
    <div className="flex gap-4 px-6 pb-6 pt-4" data-testid="property-payments-icons">
      {groups.map((group) => (
        <div key={group.label} className="flex min-w-0 flex-1 flex-col gap-2">
          <span className="text-sm leading-4 text-content">{group.label}</span>
          <div className="flex flex-wrap -space-x-3">
            {group.items.map((item) => {
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
      ))}
    </div>
  );
}
