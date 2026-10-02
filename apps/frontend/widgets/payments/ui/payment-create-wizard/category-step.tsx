'use client';

import type { JSX } from 'react';
import { RadioFalse, RadioTrue } from '@/shared/assets/icons';
import {
  CategoryIcon,
  paymentCategories,
} from '@/features/payment-categories';
import { matchesTitleSearch } from '@/features/payments';
import { ListRow } from '@/shared/ui/design';

/**
 * Список категорий из дефолтного каталога (#447; «Другое» — замыкающая
 * строка, #1069) — шаг 1
 * визарда (Figma 781:12299: кружок выбора RadioFalse/RadioTrue справа)
 * и пикер в модалке правки платежа. Строка только выбирает; переход
 * дальше — кнопка шага. Поиск фильтрует по названию (запрос поднимен
 * в хедер визарда); свои категории — следующий срез, поэтому без ветки
 * «создать».
 */

export type CategoryStepProps = {
  readonly selectedSlug: string | undefined;
  readonly onSelect: (slug: string) => void;
  /** Поисковый запрос; пустой/не задан — весь каталог. */
  readonly query?: string;
};

export function CategoryStep({
  selectedSlug,
  onSelect,
  query = '',
}: CategoryStepProps): JSX.Element {
  const visible = paymentCategories.filter((category) =>
    matchesTitleSearch(query, category.label),
  );

  return (
    <div className="flex flex-col pt-6">
      {visible.map((category) => (
        <ListRow
          key={category.slug}
          className="gap-4 py-1.5"
          leading={<CategoryIcon icon={category.icon} color={category.color} className="h-11 w-11" />}
          title={category.label}
          onSelect={() => onSelect(category.slug)}
          trailing={
            category.slug === selectedSlug ? (
              <RadioTrue className="h-6 w-6" aria-hidden />
            ) : (
              <RadioFalse className="h-6 w-6" aria-hidden />
            )
          }
        />
      ))}
      {visible.length === 0 && (
        <div className="flex flex-col items-center gap-4 pt-16">
          <img
            src="/images/payments/category-search.png"
            alt=""
            width={128}
            height={128}
            className="h-32 w-32"
          />
          <p className="max-w-[320px] text-center text-base leading-[18px] text-content-secondary">
            Ничего не нашлось
          </p>
        </div>
      )}
    </div>
  );
}
