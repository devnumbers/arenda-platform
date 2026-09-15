'use client';

import { useState, type JSX } from 'react';
import Image from 'next/image';
import { useRouter, useSearchParams } from 'next/navigation';
import { Cancel } from '@/shared/assets/icons';
import { goBack } from '@/shared/lib/navigation';
import { clientTodayIso } from '@/entities/payment';
import {
  operationsCategoryRows,
  operationsFiltersParams,
  operationsPeriodChipLabel,
  readOperationsFilters,
  resolveFilterReturnPath,
  usePropertyOperationsSummary,
  type OperationsCategoryRow,
} from '@/features/payments';
import { categoryStyle, CategoryIcon } from '@/features/payment-categories';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import {
  Button,
  Checkbox,
  IconButton,
  ListRow,
  PageContent,
  StickyBottomBar,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import { PaymentsSkeleton, PaymentsStateCard } from './payments-sections';

export type OperationsCategoriesScreenProps = {
  readonly propertyId: string;
};

/**
 * Страница «Выбрать категорию» (#477, Figma 1506-72116, 1510-74149) —
 * не попап, а отдельный маршрут. Верх закреплён: крестик + заголовок и
 * контекстные чипы (период + «Все категории») — контекст всегда перед
 * глазами; строки категорий скроллятся. Дефолт периода — весь период
 * (#676, карта #669, как на глобальных категориях #672): без явного
 * диапазона суммы за всё время, чип нейтральный «Период»; период
 * наследуется от списка через return-параметры. Строки — «иконка +
 * название + сумма за период + чекбокс», только категории с операциями
 * за период (разбивка сводки #473). «Выбрать» — закреплённая нижняя
 * панель, видна при непустом черновике или применённом фильтре (иначе
 * возврат к «Все категории» был бы недостижим); применяет выбор
 * возвратом на список (router.replace), крестик — goBack без изменений.
 * Пустой период — иллюстрация и подпись вместо строк (Figma 1518-92530,
 * #478); «Выбрать» скрыта, только пока нет ни черновика, ни применённого
 * фильтра категорий — опустевшая разбивка с непустым черновиком (период
 * сменили, строк не осталось) по-прежнему подтверждается.
 */
export function OperationsCategoriesScreen({
  propertyId,
}: OperationsCategoriesScreenProps): JSX.Element {
  const router = useRouter();
  const searchParams = useSearchParams();

  const today = clientTodayIso();
  const filters = readOperationsFilters(searchParams, today);
  // Дефолт категорий — весь период (#676, как на глобальных #672):
  // без явного выбора даты в запрос не уходят, чип нейтральный.
  const period = filters.period;
  const [draft, setDraft] = useState<ReadonlyArray<string>>(filters.categories);

  // Куда возвращаться: список, открывший страницу (только маршруты операций
  // этого объекта), иначе — главный список.
  const returnTo = resolveFilterReturnPath(searchParams.get('return'), propertyId);

  const summaryQuery = usePropertyOperationsSummary(propertyId, {
    status: 'paid',
    order: 'desc',
    dateFrom: period?.from,
    dateTo: period?.to,
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
    // Формат query — один хелпер с operationsFiltersHref (#472); черновик
    // подаётся как выбор категорий того же фильтр-шейпа.
    const query = new URLSearchParams(
      operationsFiltersParams({ period: filters.period, categories: draft }),
    );
    const queryString = query.toString();
    router.replace(`${returnTo}${queryString ? `?${queryString}` : ''}`);
  };

  return (
    <>
      {/* Каркас — как у всех страниц: TopNav (на десктопе — с «крыльями»
       * лого/профиль, закреплён наверху) + PageContent 560 + StickyBottomBar.
       * Верх закреплён: заголовок в TopNav, контекстные чипы — шапка колонки
       * (решение владельца 2026-09-02). Колонка во всю высоту вьюпорта минус
       * TopNav — строки скроллятся внутри себя, страница остаётся на месте. */}
      <TopNav leading={<IconButton icon={<Cancel />} label="Закрыть" onClick={cancel} />}>
        <TopNavTitle title="Выбрать категорию" />
      </TopNav>

      <PageContent className="flex h-[calc(100dvh-72px)] flex-col pb-0">
        <div className="flex shrink-0 flex-wrap gap-1.5 px-6">
          {/* Чип периода — дисплейный (период наследуется от ленты через
           * return-параметры): синий только с явным диапазоном, в дефолте
           * серый «Период» (#676, канон #670/#672). */}
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

        <div className="min-h-0 flex-1 overflow-y-auto pt-2 pb-[calc(6.5rem+env(safe-area-inset-bottom))]">
          {summaryQuery.isPending ? (
            <PaymentsSkeleton />
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
            // Пустой выбор категорий (Figma 1518-92530, #478): папка 128
            // через 80px после чипов, подпись 16/18 серым (320 по ширине).
            <div className="flex flex-col items-center px-6 pt-[72px]">
              <Image
                src="/images/payments/operations-categories.png"
                alt=""
                width={128}
                height={128}
                className="h-32 w-32"
              />
              <p className="mt-4 max-w-[320px] text-center text-base leading-[18px] text-content-secondary">
                Категорий, по которым были операции в этот период не было.
                Попробуйте выбрать другой период
              </p>
            </div>
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
        // «Все категории»: возврат к дефолту должен быть достижим.
        <StickyBottomBar>
          <Button className="w-full" onClick={apply} aria-label="Выбрать категории">
            Выбрать
          </Button>
        </StickyBottomBar>
      ) : null}
    </>
  );
}

