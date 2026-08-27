'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import { Check } from '@/shared/assets/icons';
import {
  CategoryIcon,
  paymentCategories,
} from '@/features/payment-categories';
import { matchesTitleSearch } from '@/features/payments';
import { ListRow, SearchField } from '@/shared/ui/design';
import { PaymentsEmptyCard } from '../payments-sections';

/**
 * Шаг 1 визарда — категория из дефолтного каталога (#447, 40 категорий)
 * с поиском по названию. Выбор сразу ведёт на шаг названия; свои категории —
 * следующий срез, поэтому поиск без ветки «создать».
 */

export type CategoryStepProps = {
  readonly selectedSlug: string | undefined;
  readonly onSelect: (slug: string) => void;
};

export function CategoryStep({
  selectedSlug,
  onSelect,
}: CategoryStepProps): JSX.Element {
  const [query, setQuery] = useState('');
  const visible = paymentCategories.filter((category) =>
    matchesTitleSearch(query, category.label),
  );

  return (
    <>
      <div className="px-6 pt-2">
        <SearchField
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          onClear={() => setQuery('')}
          placeholder="Найти категорию"
          aria-label="Поиск по названиям категорий"
        />
      </div>
      <div className="flex flex-col pt-2">
        {visible.map((category) => (
          <ListRow
            key={category.slug}
            leading={<CategoryIcon icon={category.icon} color={category.color} className="h-11 w-11" />}
            title={category.label}
            onSelect={() => onSelect(category.slug)}
            trailing={
              category.slug === selectedSlug ? (
                <Check className="h-5 w-5 text-primary" aria-hidden />
              ) : undefined
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
    </>
  );
}
