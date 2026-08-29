'use client';

import type { JSX } from 'react';
import { RadioFalse, RadioTrue } from '@/shared/assets/icons';
import {
  CategoryIcon,
  paymentCategories,
} from '@/features/payment-categories';
import { matchesTitleSearch } from '@/features/payments';
import { ListRow } from '@/shared/ui/design';
import { PaymentsEmptyCard } from '../payments-sections';

/**
 * Список категорий из дефолтного каталога (#447, 40 категорий) — шаг 1
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
        <div className="pt-8">
          <PaymentsEmptyCard
            title="Ничего не нашлось"
            hint="Попробуйте изменить запрос"
          />
        </div>
      )}
    </div>
  );
}
