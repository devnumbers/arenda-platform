'use client';

import { useEffect, useRef, useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { buildReturnUrl, goBack } from '@/shared/lib/navigation';
import { useSearchQueryState } from '@/shared/lib/hooks/useSearchQueryState';
import { formatDayMonthWithYear } from '@/entities/payment';
import { dateToIsoLocal } from '@/shared/lib/calendar';
import {
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
  InfiniteQueryTail,
  PageContent,
  SearchField,
  TopNav,
} from '@/shared/ui/design';
import { OperationRow } from './operations-list';
import { PaymentsHeading, PaymentsStateCard } from './payments-sections';
import { SearchResultsSkeleton } from './operations-skeletons';
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
 * с фильтрами из адреса: объекты (#542) и период — дефолт весь период
 * (#673, как на лентах #670/#671): без явного диапазона даты в запрос
 * не уходят, поиск ищет за всё время (прецедент — объектный поиск).
 * Пустые состояния и скелетоны — как объектный поиск.
 * Ввод живёт в адресе (?q=) и догоняется дебаунсом — useSearchQueryState.
 * Шапка — канон поиска: TopNav варианта search (#564), «Назад» закрывает
 * поиск возвратом на ленту, фильтры ленты при этом сохраняются, поле
 * получает программный фокус.
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

  const today = dateToIsoLocal(new Date());

  const summaryQuery = useGlobalOperationsSummary(
    {
      ...globalSearchSummaryScope(filters.period, filters.propertyIds, debounced),
      includeArchived: filters.archived,
    },
    { enabled: debounced !== '' },
  );

  const chips = searchCategoryChips(summaryQuery.data?.categories ?? [], selectedSlug);
  // Выбор валиден, только пока чип есть в разбивке текущего запроса: после
  // смены запроса исчезнувшая категория молча перестаёт сужать список.
  const effectiveSlug = chips.some((chip) => chip.selected) ? selectedSlug : null;

  const listQuery = useGlobalOperationsPaged(
    {
      ...globalSearchListScope(filters.period, filters.propertyIds, debounced, effectiveSlug),
      includeArchived: filters.archived,
    },
    { enabled: debounced !== '' },
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
    <>
      {/* Канон поиска (TopNav варианта search): «Назад» на ленту — goBack
       * по истории, прямой загрузке — на ленту с фильтрами из адреса поиска
       * (buildReturnUrl мержит без коллизии «?»). */}
      <TopNav
        variant="search"
        leading={<IconButton icon={<ArrowLeft />} label="Назад" onClick={closeSearch} />}
      >
        <SearchField
          ref={inputRef}
          placeholder="Найти операцию"
          aria-label="Найти операцию"
          value={value}
          onChange={(event) => setValue(event.target.value)}
          onClear={clear}
        />
      </TopNav>

      <PageContent>
        {emptyQuery && (
          <EmptyState
            imageSrc="/images/payments/operations-search.png"
            className="py-16"
            description="Введите название операции, сумму, категорию"
          />
        )}

        {!emptyQuery && listQuery.isError && (
          <div className="pt-4">
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
          </div>
        )}

        {pending && (
          // Паритет §7/#604: скелетон зеркалит две секции контента — чипы
          // категорий и строки операций (дата под суммой, #543).
          <SearchResultsSkeleton description />
        )}

        {showResults &&
          (operations.length === 0 ? (
            <EmptyState
              imageSrc="/images/payments/operations-search.png"
              className="py-16"
              description="Такой операции нет"
            />
          ) : (
            <div className="flex flex-col gap-6 pt-4">
              {chips.length > 0 && (
                <section aria-label="Категории">
                  <PaymentsHeading>Категории</PaymentsHeading>
                  <div className="flex flex-wrap gap-1.5 px-6 pt-2">
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
                <PaymentsHeading>Операции</PaymentsHeading>
                <div className="flex flex-col pt-2">
                  {operations.map((operation) => (
                    <OperationRow
                      key={operation.id}
                      operation={operation}
                      subtitle={operation.propertyName}
                      description={formatDayMonthWithYear(operation.date, today)}
                      onSelect={() => openOperation(operation)}
                    />
                  ))}
                  <InfiniteQueryTail query={listQuery} />
                </div>
              </section>
            </div>
          ))}
      </PageContent>
    </>
  );
}
