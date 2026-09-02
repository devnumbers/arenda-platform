'use client';

import { useState, type JSX } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { Cancel } from '@/shared/assets/icons';
import { goBack } from '@/shared/lib/navigation';
import { clientTodayIso } from '@/entities/payment';
import {
  defaultOperationsPeriod,
  operationsCategoryRows,
  operationsPeriodRangeChipLabel,
  readOperationsFilters,
  resolveFilterReturnPath,
  usePropertyOperationsSummary,
  type OperationsCategoryRow,
} from '@/features/payments';
import { categoryStyle, CategoryIcon } from '@/features/payment-categories';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import { Button, Checkbox, IconButton, ListRow, StickyBottomBar } from '@/shared/ui/design';
import { PaymentsSkeleton, PaymentsStateCard } from './payments-sections';

export type OperationsCategoriesScreenProps = {
  readonly propertyId: string;
};

/**
 * Страница «Выбрать категорию» (#477, Figma 1506-72116, 1510-74149) —
 * не попап, а отдельный маршрут. Верх закреплён: крестик + заголовок и
 * контекстные чипы (период в формате диапазона + «Все категории») —
 * контекст всегда перед глазами; строки категорий скроллятся. Строки —
 * «иконка + название + сумма за период + чекбокс», только категории
 * с операциями за период (разбивка сводки #473). «Выбрать» — закреплённая
 * нижняя панель, видна при непустом черновике или применённом фильтре
 * (иначе возврат к «Все категории» был бы недостижим); применяет выбор
 * возвратом на список (router.replace), крестик — goBack без изменений.
 */
export function OperationsCategoriesScreen({
  propertyId,
}: OperationsCategoriesScreenProps): JSX.Element {
  const router = useRouter();
  const searchParams = useSearchParams();

  const today = clientTodayIso();
  const filters = readOperationsFilters(searchParams, today);
  const period = filters.period ?? defaultOperationsPeriod(today);
  const [draft, setDraft] = useState<ReadonlyArray<string>>(filters.categories);

  // Куда возвращаться: список, открывший страницу (только маршруты операций
  // этого объекта), иначе — главный список.
  const returnTo = resolveFilterReturnPath(searchParams.get('return'), propertyId);

  const summaryQuery = usePropertyOperationsSummary(propertyId, {
    status: 'paid',
    order: 'desc',
    dateFrom: period.from,
    dateTo: period.to,
  });
  const rows: ReadonlyArray<OperationsCategoryRow> = operationsCategoryRows(
    summaryQuery.data?.categories ?? [],
  );

  // Черновик живёт от монтирования до монтирования: страница монтируется
  // заново на каждый вход, useState инициализируется применённым выбором.
  const toggle = (slug: string): void => {
    setDraft(
      draft.includes(slug) ? draft.filter((candidate) => candidate !== slug) : [...draft, slug],
    );
  };

  const cancel = (): void => {
    const params = new URLSearchParams(searchParams);
    params.delete('return');
    const queryString = params.toString();
    goBack(router, `${returnTo}${queryString ? `?${queryString}` : ''}`);
  };

  const apply = (): void => {
    const query = new URLSearchParams();
    if (filters.period !== null) {
      query.set('from', filters.period.from);
      query.set('to', filters.period.to);
    }
    if (draft.length > 0) {
      query.set('category', draft.join(','));
    }
    const queryString = query.toString();
    router.replace(`${returnTo}${queryString ? `?${queryString}` : ''}`);
  };

  return (
    <div className="flex h-dvh flex-col bg-white font-sans desktop:mx-auto desktop:max-w-[560px]">
      {/* Закреплённый верх: заголовок и контекстные чипы (решение владельца
       * 2026-09-02). */}
      <header className="relative flex h-14 shrink-0 items-center justify-center">
        <div className="absolute left-0 top-0 flex h-full items-center pl-2">
          <IconButton icon={<Cancel />} label="Закрыть" onClick={cancel} />
        </div>
        <span className="text-base font-medium leading-[18px] text-content">
          Выбрать категорию
        </span>
      </header>

      <div className="flex shrink-0 flex-wrap gap-1.5 px-6 pb-3">
        <span
          aria-hidden
          className="inline-flex h-11 items-center rounded-pill bg-primary px-5 text-sm font-medium text-white"
        >
          {operationsPeriodRangeChipLabel(period)}
        </span>
        <span
          aria-hidden
          className="inline-flex h-11 items-center rounded-pill bg-surface-muted px-5 text-sm font-medium text-content"
        >
          Все категории
        </span>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto pb-[calc(6.5rem+env(safe-area-inset-bottom))]">
        {summaryQuery.isPending ? (
          <div className="pt-2">
            <PaymentsSkeleton />
          </div>
        ) : summaryQuery.isError ? (
          <div className="pt-2">
            <PaymentsStateCard
              title="Не удалось загрузить категории"
              hint="Проверьте подключение и попробуйте еще раз"
              action={
                <Button
                  variant="secondary"
                  size="small"
                  onClick={() => void summaryQuery.refetch()}
                >
                  Повторить
                </Button>
              }
            />
          </div>
        ) : rows.length === 0 ? (
          <p className="px-6 py-8 text-center text-base leading-[18px] text-content-secondary">
            В этом периоде нет операций
          </p>
        ) : (
          <div className="flex flex-col">
            {rows.map((row) => {
              const style = categoryStyle('default', row.slug);
              return (
                <ListRow
                  key={row.slug}
                  leading={<CategoryIcon icon={style.icon} color={style.color} />}
                  title={row.label}
                  value={formatMoneyKopecks(row.totalKopecks)}
                  trailing={
                    // Клик по чекбоксу не должен дощёлкивать до строки —
                    // иначе toggle сработает дважды (чекбокс + строка).
                    <Checkbox
                      aria-label={`Категория ${row.label}`}
                      checked={draft.includes(row.slug)}
                      onCheckedChange={() => toggle(row.slug)}
                      onClick={(event) => event.stopPropagation()}
                    />
                  }
                  onSelect={() => toggle(row.slug)}
                />
              );
            })}
          </div>
        )}
      </div>

      {draft.length > 0 || filters.categories.length > 0 ? (
        // Пустой черновик при применённом фильтре — легитимное применение
        // «Все категории»: возврат к дефолту должен быть достижим.
        <StickyBottomBar>
          <Button className="w-full" onClick={apply} aria-label="Выбрать категории">
            Выбрать
          </Button>
        </StickyBottomBar>
      ) : null}
    </div>
  );
}
