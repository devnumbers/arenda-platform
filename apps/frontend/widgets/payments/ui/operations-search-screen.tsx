'use client';

import { useEffect, useRef, useState, type JSX } from 'react';
import Image from 'next/image';
import { useRouter } from 'next/navigation';
import { ArrowLeft } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { useSearchQueryState } from '@/shared/lib/hooks/useSearchQueryState';
import { formatDayMonthWithYear } from '@/entities/payment';
import { dateToIsoLocal } from '@/shared/lib/calendar';
import {
  usePropertyOperationsScopedPaged,
  usePropertyOperationsSummary,
} from '@/features/payments';
import {
  Button,
  ChipButton,
  IconButton,
  InfiniteQueryTail,
  PageContent,
  SearchField,
  TopNav,
} from '@/shared/ui/design';
import { OperationRow } from './operations-list';
import { PaymentsHeading, PaymentsStateCard } from './payments-sections';
import { OperationsSearchSkeleton } from './operations-skeletons';
import {
  searchCategoryChips,
  searchListScope,
  searchSummaryScope,
} from '../lib/operations-search-model';

/**
 * Экран поиска операций объекта (#476, Figma 1494-61633/61657/61679/63035):
 * хедер-поиск — TopNav варианта search (назад + SearchField «Найти
 * операцию» с крестиком очистки). Серверная область запроса — подсказка
 * Figma «Введите название операции, сумму, категорию»: контракт `search`
 * ищет по названию, категории и (числовой запрос) по сумме; скоуп — только
 * оплаченные операции без ограничения периода (решение карты #472).
 * Результаты — чипы совпавших категорий (разбивка сводки тем же предикатом;
 * тап сужает список, повторный снимает) и список операций новыми сверху
 * с датой в подзаголовке, порции по 50 с бесконечным скроллом. Пустые
 * состояния: ничего не введено — «Введите название операции, сумму,
 * категорию», нет совпадений — «Такой операции нет». Ввод живёт в адресе
 * (?q=) и догоняется дебаунсом — useSearchQueryState.
 */
export function OperationsSearchScreen({
  propertyId,
}: {
  readonly propertyId: string;
}): JSX.Element {
  const router = useRouter();
  const { value, debounced, setValue, clear } = useSearchQueryState();
  const [selectedSlug, setSelectedSlug] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  // Полевой фокус при входе на экран (Figma 1494-61633 — каретка в поле);
  // jsx-a11y/no-autofocus запрещает сам проп.
  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  const summaryQuery = usePropertyOperationsSummary(
    propertyId,
    searchSummaryScope(debounced),
    { enabled: debounced !== '' },
  );

  const chips = searchCategoryChips(summaryQuery.data?.categories ?? [], selectedSlug);
  // Выбор валиден, только пока чип есть в разбивке текущего запроса: после
  // смены запроса исчезнувшая категория молча перестаёт сужать список.
  const effectiveSlug = chips.some((chip) => chip.selected) ? selectedSlug : null;

  const listQuery = usePropertyOperationsScopedPaged(
    propertyId,
    searchListScope(debounced, effectiveSlug),
    { enabled: debounced !== '' },
  );

  const today = dateToIsoLocal(new Date());
  const operations = listQuery.data ?? [];

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

  const openOperation = (operation: { readonly id: string }): void =>
    router.push(ROUTES.propertyOperation(propertyId, operation.id));

  const toggleChip = (slug: string): void =>
    setSelectedSlug((current) => (current === slug ? null : slug));

  return (
    <>
      <TopNav
        variant="search"
        leading={
          <IconButton
            icon={<ArrowLeft />}
            label="Назад"
            // Назад — на главный список операций, не по истории браузера.
            onClick={() => router.push(ROUTES.propertyOperations(propertyId))}
          />
        }
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
          <SearchStateView text="Введите название операции, сумму, категорию" />
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
          // категорий и строки операций.
          <OperationsSearchSkeleton />
        )}

        {showResults &&
          (operations.length === 0 ? (
            <SearchStateView text="Такой операции нет" />
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
                      subtitle={formatDayMonthWithYear(operation.date, today)}
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

/** Иллюстрированное состояние поиска (Figma 1494-61633 и 1495-63035):
 * иллюстрация 128 и одна строка 16/18 серым — и «ничего не введено», и
 * «нет совпадений» (в макетах одна иллюстрация). */
function SearchStateView({ text }: { readonly text: string }): JSX.Element {
  return (
    <div className="flex flex-col items-center gap-4 py-16">
      <Image
        src="/images/payments/operations-search.png"
        alt=""
        width={128}
        height={128}
        className="h-32 w-32"
      />
      <p className="max-w-[280px] text-center text-base leading-[18px] text-content-secondary">
        {text}
      </p>
    </div>
  );
}
