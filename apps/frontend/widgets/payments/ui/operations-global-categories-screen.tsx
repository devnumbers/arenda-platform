'use client';

import { useState, type JSX } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { buildReturnUrl } from '@/shared/lib/navigation';
import { clientTodayIso } from '@/entities/payment';
import {
  defaultOperationsPeriod,
  globalOperationsFiltersParams,
  operationsCategoryRows,
  operationsPeriodRangeChipLabel,
  readGlobalOperationsFilters,
  resolveGlobalFilterReturnPath,
  useGlobalOperationsSummary,
} from '@/features/payments';
import { categoryStyle, CategoryIcon } from '@/features/payment-categories';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import {
  Button,
  Checkbox,
  EmptyState,
  ListRow,
  PageContent,
  StickyBottomBar,
} from '@/shared/ui/design';
import { PageHeader } from '@/shared/ui/page-header';
import { PaymentsSkeleton, PaymentsStateCard } from './payments-sections';
import { globalCategoriesSummaryScope } from '../lib/operations-global-categories-model';

/**
 * Страница «Выбрать категорию» глобальной ленты (#544) — копия объектной
 * страницы (#477, Figma 1506-72116) по всем видимым объектам: чипы контекста
 * (период диапазоном + «Все категории»), строки «иконка + название + сумма
 * периода + чекбокс» из разбивки глобальной сводки (#540) — только категории
 * с операциями в скоупе. Строгий черновик: тапы меняют подсветку, применяются
 * «Выбрать» (возврат на список, router.replace), «назад» отбрасывает; период
 * и объекты (#542) переживают применение — categories мержится в return через
 * buildReturnUrl + globalOperationsFiltersParams (единый wire-формат с лентой).
 * Кнопка видна при непустом черновике или применённом фильтре (иначе возврат
 * к «Все категории» был бы недостижим); пустой период — EmptyState (Figma
 * 1518-92530, #478), «Выбрать» в этом состоянии не нужен. Каркас кабинетный
 * (решение #542: зона /operations без TopNav — конфликт с сайдбаром),
 * ритм 24px всей страницей (#541); панель приподнята над нижней навигацией
 * кабинета ниже 1200.
 */
export function OperationsGlobalCategoriesScreen(): JSX.Element {
  const router = useRouter();
  const searchParams = useSearchParams();

  const today = clientTodayIso();
  const filters = readGlobalOperationsFilters(searchParams, today);
  const period = filters.period ?? defaultOperationsPeriod(today);
  // Черновик живёт от монтирования до монтирования: страница монтируется
  // заново на каждый вход, useState инициализируется применённым выбором.
  const [draft, setDraft] = useState<ReadonlyArray<string>>(filters.categories);

  // Куда возвращаться: список зоны операций, открывший страницу, иначе —
  // главный список.
  const returnTo = resolveGlobalFilterReturnPath(searchParams.get('return'));

  const summaryQuery = useGlobalOperationsSummary(
    globalCategoriesSummaryScope(period, filters.propertyIds),
  );
  const rows = operationsCategoryRows(summaryQuery.data?.categories ?? []);

  const toggle = (slug: string): void => {
    setDraft(
      draft.includes(slug) ? draft.filter((candidate) => candidate !== slug) : [...draft, slug],
    );
  };

  const apply = (): void => {
    // Единая сериализация фильтров ленты поверх returnTo с сохранением его
    // query — buildReturnUrl мержит параметры без коллизии «?».
    router.replace(
      buildReturnUrl(returnTo, globalOperationsFiltersParams({ ...filters, categories: draft })),
    );
  };

  return (
    <>
      <PageHeader title="Выбрать категорию" backHref={returnTo} />

      {/* Ритм страницы — ровно 24px по бокам, как на ленте (#541): чипы и
       * строки прижаты к этому краю без своих вставок. */}
      <PageContent>
        <div className="-mx-5 flex min-[1200px]:mx-0 flex-col gap-6 px-6 pt-1">
          <div className="flex flex-wrap gap-1.5">
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

          {summaryQuery.isPending ? (
            <PaymentsSkeleton withHeading />
          ) : summaryQuery.isError ? (
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
          ) : rows.length === 0 ? (
            // Пустой период (Figma 1518-92530, #478) — канон EmptyState.
            <EmptyState
              imageSrc="/images/payments/operations-categories.png"
              className="py-16"
              description="Категорий, по которым были операции в этот период не было. Попробуйте выбрать другой период"
            />
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
      </PageContent>

      {draft.length > 0 || filters.categories.length > 0 ? (
        // Пустой черновик при применённом фильтре — легитимное применение
        // «Все категории»: возврат к дефолту должен быть достижим. Ниже 1200
        // в кабинете видна нижняя навигация — панель приподнимается над ней.
        <StickyBottomBar className="max-[1199px]:bottom-[calc(5rem_+_env(safe-area-inset-bottom))]">
          <Button className="w-full" onClick={apply} aria-label="Выбрать категории">
            Выбрать
          </Button>
        </StickyBottomBar>
      ) : null}
    </>
  );
}
