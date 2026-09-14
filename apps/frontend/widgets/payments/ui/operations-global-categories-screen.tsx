'use client';

import { useState, type JSX } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { Cancel } from '@/shared/assets/icons';
import { buildReturnUrl, goBack } from '@/shared/lib/navigation';
import { clientTodayIso } from '@/entities/payment';
import {
  globalOperationsFiltersParams,
  operationsCategoryRows,
  operationsPeriodChipLabel,
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
  IconButton,
  ListRow,
  PageContent,
  StickyBottomBar,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import { PaymentsStateCard } from './payments-sections';
import { OperationsCategoriesSkeleton } from './operations-skeletons';
import { globalCategoriesSummaryScope } from '../lib/operations-global-categories-model';

/**
 * Страница «Выбрать категорию» глобальной ленты (#544) — копия объектной
 * страницы (#477, Figma 1506-72116) по всем видимым объектам: чипы контекста
 * (период диапазоном + «Все категории»), строки «иконка + название + сумма
 * периода + чекбокс» из разбивки глобальной сводки (#540) — только категории
 * с операциями в скоупе. Дефолт периода — весь период (#672, как на лентах
 * #670/#671): без явного диапазона суммы за всё время, чип нейтральный.
 * Строгий черновик: тапы меняют подсветку, применяются
 * «Выбрать» (возврат на список, router.replace), «назад» отбрасывает; период
 * и объекты (#542) переживают применение — categories мержится в return через
 * buildReturnUrl + globalOperationsFiltersParams (единый wire-формат с лентой).
 * Кнопка видна при непустом черновике или применённом фильтре (иначе возврат
 * к «Все категории» был бы недостижим); пустая разбивка — EmptyState
 * (Figma 1518-92530, #478). Шапка — канон
 * выборщика (как у объектной страницы: ✕ «Закрыть» + заголовок в TopNav,
 * #564).
 */
export function OperationsGlobalCategoriesScreen(): JSX.Element {
  const router = useRouter();
  const searchParams = useSearchParams();

  const today = clientTodayIso();
  const filters = readGlobalOperationsFilters(searchParams, today);
  // Дефолт периода страницы — весь период (#672): даты в запросе только
  // с явным выбором.
  const period = filters.period;
  // Черновик живёт от монтирования до монтирования: страница монтируется
  // заново на каждый вход, useState инициализируется применённым выбором.
  const [draft, setDraft] = useState<ReadonlyArray<string>>(filters.categories);

  // Куда возвращаться: список зоны операций, открывший страницу, иначе —
  // главный список.
  const returnTo = resolveGlobalFilterReturnPath(searchParams.get('return'));

  const summaryQuery = useGlobalOperationsSummary({
    ...globalCategoriesSummaryScope(period, filters.propertyIds),
    includeArchived: filters.archived,
  });
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
      {/* Канон выборщика (как у объектной страницы #477): ✕ «Закрыть»
       * отбрасывает черновик (goBack — по истории, прямой загрузке —
       * на returnTo). */}
      <TopNav
        leading={<IconButton icon={<Cancel />} label="Закрыть" onClick={() => goBack(router, returnTo)} />}
      >
        <TopNavTitle title="Выбрать категорию" />
      </TopNav>

      <PageContent>
        <div className="flex flex-col gap-6 pt-4">
          <div className="flex flex-wrap gap-1.5 px-6">
            {/* Чип периода — дисплейный (период наследуется от ленты
             * через return-параметры): синий только с явным диапазоном,
             * в дефолте серый «Период» (#672, канон #670). */}
            <span
              aria-hidden
              className={`inline-flex h-11 items-center rounded-pill px-5 text-sm font-medium ${
                period !== null ? 'bg-primary text-white' : 'bg-surface-muted text-content'
              }`}
            >
              {operationsPeriodChipLabel(period)}
            </span>
            <span
              aria-hidden
              className="inline-flex h-11 items-center rounded-pill bg-surface-muted px-5 text-sm font-medium text-content"
            >
              Все категории
            </span>
          </div>

          {summaryQuery.isPending ? (
            // Паритет §7: строки «иконка + название + сумма + чекбокс»;
            // чипы контекста выше — вне фазы загрузки.
            <OperationsCategoriesSkeleton />
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
            // Пустая разбивка (Figma 1518-92530, #478) — канон EmptyState.
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
        // «Все категории»: возврат к дефолту должен быть достижим. TabBar
        // глушится сам, пока смонтирована панель (#564).
        <StickyBottomBar>
          <Button className="w-full" onClick={apply} aria-label="Выбрать категории">
            Выбрать
          </Button>
        </StickyBottomBar>
      ) : null}
    </>
  );
}
