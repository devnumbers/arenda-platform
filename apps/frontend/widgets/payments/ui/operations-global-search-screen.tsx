'use client';

import { useEffect, useRef, useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { buildReturnUrl, goBack } from '@/shared/lib/navigation';
import { useInfiniteScroll } from '@/shared/lib/hooks/useInfiniteScroll';
import { useSearchQueryState } from '@/shared/lib/hooks/useSearchQueryState';
import { clientTodayIso, formatDayMonthWithYear } from '@/entities/payment';
import {
  defaultOperationsPeriod,
  globalOperationsFiltersParams,
  useGlobalOperationsFilters,
  useGlobalOperationsPaged,
  useGlobalOperationsSummary,
} from '@/features/payments';
import {
  Button,
  ChipButton,
  EmptyState,
  IconButton,
  PageContent,
  SearchField,
} from '@/shared/ui/design';
import { LoadingMoreIndicator, OperationRow } from './operations-list';
import { PaymentsHeading, PaymentsSkeleton, PaymentsStateCard } from './payments-sections';
import {
  globalSearchListScope,
  globalSearchSummaryScope,
  searchCategoryChips,
} from '../lib/operations-search-model';

/**
 * Поиск глобальных операций (#543, Figma 1726-90433) — копия объектного
 * поиска (#476) с подзаголовком-объектом в строках: шапка «назад» + поле
 * «Найти операцию» с крестиком очистки, чипы совпавших категорий (разбивка
 * сводки тем же предикатом; тап сужает список, повторный снимает) и список
 * операций новыми сверху: иконка, название, подзаголовок-объект, справа
 * сумма и дата (макет 1726-90433); тап — на страницу операции своего
 * объекта. Серверная область — контракт `search` глобальной ленты (#540)
 * с фильтрами из адреса: объекты (#542) и период (дефолт — текущий месяц,
 * как на ленте #541). Пустые состояния и скелетоны — как объектный поиск.
 * Ввод живёт в адресе (?q=) и догоняется дебаунсом — useSearchQueryState.
 * Каркас кабинетный (решение #542: зона /operations без TopNav — конфликт
 * с сайдбаром): шапка в потоке страницы, «назад» закрывает поиск возвратом
 * на ленту, фильтры ленты при этом сохраняются.
 */
export function OperationsGlobalSearchScreen(): JSX.Element {
  const router = useRouter();
  const { value, debounced, setValue, clear } = useSearchQueryState();
  const { filters } = useGlobalOperationsFilters();
  const [selectedSlug, setSelectedSlug] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  // Полевой фокус при входе на экран (поисковая шапка — каретка в поле);
  // jsx-a11y/no-autofocus запрещает сам проп.
  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  const today = clientTodayIso();
  const period = filters.period ?? defaultOperationsPeriod(today);

  const summaryQuery = useGlobalOperationsSummary(
    globalSearchSummaryScope(period, filters.propertyIds, debounced),
    { enabled: debounced !== '' },
  );

  const chips = searchCategoryChips(summaryQuery.data?.categories ?? [], selectedSlug);
  // Выбор валиден, только пока чип есть в разбивке текущего запроса: после
  // смены запроса исчезнувшая категория молча перестаёт сужать список.
  const effectiveSlug = chips.some((chip) => chip.selected) ? selectedSlug : null;

  const listQuery = useGlobalOperationsPaged(
    globalSearchListScope(period, filters.propertyIds, debounced, effectiveSlug),
    { enabled: debounced !== '' },
  );

  const sentinelRef = useInfiniteScroll(
    () => {
      if (listQuery.hasNextPage && !listQuery.isFetchingNextPage) {
        void listQuery.fetchNextPage();
      }
    },
    listQuery.hasNextPage === true,
  );

  const operations = listQuery.data ?? [];

  const closeSearch = (): void =>
    // По истории — на ленту, откуда пришли; прямая загрузка — на ленту с
    // фильтрами из адреса поиска (buildReturnUrl мержит без коллизии «?»).
    goBack(
      router,
      buildReturnUrl(ROUTES.operations, globalOperationsFiltersParams(filters)),
    );

  const openOperation = (operation: {
    readonly propertyId: string;
    readonly id: string;
  }): void => router.push(ROUTES.propertyOperation(operation.propertyId, operation.id));

  const toggleChip = (slug: string): void =>
    setSelectedSlug((current) => (current === slug ? null : slug));

  const emptyQuery = value.trim() === '';
  // Скелетон — только пока данных нет вовсе (первый запрос): правка запроса
  // держит прежние результаты (keepPreviousData) и не дёргает экран; ошибка
  // без данных показывает карточку повтора, не скелетон.
  const pending =
    !emptyQuery
    && (listQuery.data === undefined || summaryQuery.data === undefined)
    && !listQuery.isError
    && !summaryQuery.isError;
  const showResults = !emptyQuery && !pending && !listQuery.isError;

  return (
    <PageContent>
      {/* Ритм страницы — ровно 24px по бокам, как на ленте (#541): шапка,
       * чипы, состояния и строки прижаты к этому краю без своих вставок. */}
      <div className="-mx-5 flex min-[1200px]:mx-0 flex-col gap-6 px-6 pt-1">
        <div className="flex items-center gap-1">
          <IconButton icon={<ArrowLeft />} label="Назад" onClick={closeSearch} />
          <SearchField
            ref={inputRef}
            placeholder="Найти операцию"
            aria-label="Найти операцию"
            value={value}
            onChange={(event) => setValue(event.target.value)}
            onClear={clear}
          />
        </div>

        {emptyQuery && (
          <EmptyState
            imageSrc="/images/payments/operations-search.png"
            className="py-16"
            description="Введите название операции, сумму, категорию"
          />
        )}

        {!emptyQuery && listQuery.isError && (
          <PaymentsStateCard
            title="Не удалось загрузить результаты"
            hint="Проверьте подключение и попробуйте еще раз"
            action={
              <Button
                variant="secondary"
                size="small"
                onClick={() => void listQuery.refetch()}
              >
                Повторить
              </Button>
            }
          />
        )}

        {pending && (
          <>
            <PaymentsSkeleton withHeading />
            <PaymentsSkeleton withHeading />
          </>
        )}

        {showResults &&
          (operations.length === 0 ? (
            <EmptyState
              imageSrc="/images/payments/operations-search.png"
              className="py-16"
              description="Такой операции нет"
            />
          ) : (
            <div className="flex flex-col gap-6">
              {chips.length > 0 && (
                <section aria-label="Категории">
                  <PaymentsHeading inset={false}>Категории</PaymentsHeading>
                  <div className="flex flex-wrap gap-1.5 pt-2">
                    {chips.map((chip) => (
                      <ChipButton
                        key={chip.slug}
                        selected={chip.selected}
                        aria-label={`Категория ${chip.label}`}
                        onClick={() => toggleChip(chip.slug)}
                      >
                        {chip.label}
                      </ChipButton>
                    ))}
                  </div>
                </section>
              )}

              <section aria-label="Операции">
                <PaymentsHeading inset={false}>Операции</PaymentsHeading>
                <div className="flex flex-col pt-2">
                  {operations.map((operation) => (
                    <OperationRow
                      key={operation.id}
                      operation={operation}
                      subtitle={operation.propertyName}
                      description={formatDayMonthWithYear(operation.date, today)}
                      className="-mx-3 px-0 py-3"
                      onSelect={() => openOperation(operation)}
                    />
                  ))}
                  {listQuery.hasNextPage === true && <div ref={sentinelRef} aria-hidden />}
                  {listQuery.isFetchingNextPage && <LoadingMoreIndicator />}
                </div>
              </section>
            </div>
          ))}
      </div>
    </PageContent>
  );
}
