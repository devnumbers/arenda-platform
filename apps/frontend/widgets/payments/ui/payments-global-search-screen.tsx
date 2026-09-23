'use client';

import { useEffect, useRef, useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft, Star } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { useSearchQueryState } from '@/shared/lib/hooks/useSearchQueryState';
import { useGlobalPaymentSearch, useGlobalPaymentSearchCategories } from '@/features/payments';
import type { GlobalPayment } from '@/entities/payment';
import { PaymentRowButton } from '@/entities/payment';
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
import { GlobalPaymentRuleIcon, PaymentsHeading, PaymentsSkeleton, PaymentsStateCard } from './payments-sections';
import { nearestDateLine } from '../lib/payments-global-model';
import {
  effectiveChipKey,
  searchCategoryChipKey,
  searchCategoryChips,
  searchChipFilter,
} from '../lib/payments-search-model';

/**
 * Поиск платежей (#581, Figma 706:12168/12649, 862:24128, 860:22842) —
 * поисковая шапка соседей (контакты #508, операции #543): «назад» + поле
 * «Поиск платежа». Старт — «Введите название платежа», пустой результат —
 * «Такого платежа нет» (одна иллюстрация). Результаты: чипы совпавших
 * категорий (серверные matchedCategories #575; тап сужает список
 * серверным фильтром категории — направление в чип не входит, одна
 * категория — один чип #602; повторный тап снимает — выбранная
 * синяя заливка; чипы при выборе не сужаются — канон операций) и
 * «Платежи» строками канона PaymentRowButton — как в списках карты
 * (звезда-индикатор только у избранных, пассивная — решение владельца
 * #580). Список — порциями по 50 с догрузкой при скролле, как во всех
 * лентах операций (доработка #581: «Показать все» снято владельцем). Тап
 * строке — страница платежа. Серверная область — GET /payments/search
 * (#575); ввод живёт в адресе (?q=) и догоняется дебаунсом —
 * useSearchQueryState. Шапка — канон поиска TopNav variant="search" на
 * едином хроме экранов (#565), как у операций (#543) и контактов (#508);
 * «назад» возвращает на главный «Платежи».
 */
export function PaymentsGlobalSearchScreen(): JSX.Element {
  const router = useRouter();
  const { value, debounced, setValue, clear } = useSearchQueryState();
  const [selectedKey, setSelectedKey] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  // Полевой фокус при входе на экран (поисковая шапка — каретка в поле);
  // jsx-a11y/no-autofocus запрещает сам проп.
  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  // Чипы — из лёгкого отдельного запроса (сервер считает matchedCategories
  // по всему скоупу), список — из бесконечного с серверным фильтром чипа:
  // разрыв цикла «фильтр нужен до запроса, чипы — из его ответа».
  const categoriesQuery = useGlobalPaymentSearchCategories(debounced, {
    enabled: debounced.trim() !== '',
  });

  const matched = categoriesQuery.data ?? [];
  const chips = searchCategoryChips(matched, selectedKey);
  // Выбор валиден, только пока чип есть в выдаче текущего запроса; фильтр
  // уезжает на сервер категорией чипа (направления в контракте нет — #602).
  const activeKey = effectiveChipKey(matched, selectedKey);
  const activeChip =
    activeKey !== null
      ? matched.find((chip) => searchCategoryChipKey(chip) === activeKey)
      : undefined;

  const searchQuery = useGlobalPaymentSearch(
    debounced,
    activeChip !== undefined ? searchChipFilter(activeChip) : {},
    { enabled: debounced.trim() !== '' },
  );

  const items = searchQuery.data?.items ?? [];

  const emptyQuery = value.trim() === '';
  // Скелетон — только пока данных нет вовсе (первый запрос): правка запроса
  // держит прежние результаты (keepPreviousData) и не дёргает экран; ошибка
  // без данных показывает карточку повтора, не скелетон.
  const pending =
    !emptyQuery
    && (searchQuery.data === undefined || categoriesQuery.data === undefined)
    && !searchQuery.isError
    && !categoriesQuery.isError;
  const showResults = !emptyQuery && !pending && !searchQuery.isError && !categoriesQuery.isError;

  const toggleChip = (key: string): void =>
    setSelectedKey((current) => (current === key ? null : key));

  const openPayment = (payment: GlobalPayment): void =>
    router.push(ROUTES.propertyPayment(payment.propertyId, payment.id));

  return (
    <>
      {/* Канон поиска (TopNav варианта search): «Назад» — goBack на главный
       * «Платежи»; хедер-хром (лого/профиль) приносят крылья TopNav. */}
      <TopNav
        variant="search"
        leading={<IconButton icon={<ArrowLeft />} label="Назад" onClick={() => goBack(router, ROUTES.payments)} />}
      >
        <SearchField
          ref={inputRef}
          placeholder="Поиск платежа"
          aria-label="Поиск платежа"
          value={value}
          onChange={(event) => setValue(event.target.value)}
          onClear={clear}
        />
      </TopNav>

      <PageContent>
        {emptyQuery && (
          <EmptyState
            imageSrc="/images/payments/payments-search.png"
            className="py-16"
            description="Введите название платежа"
          />
        )}

        {!emptyQuery && (searchQuery.isError || categoriesQuery.isError) && (
          <PaymentsStateCard
            title="Не удалось загрузить результаты"
            hint="Проверьте подключение и попробуйте еще раз"
            action={
              <Button
                variant="secondary"
                size="small"
                onClick={() => void searchQuery.refetch()}
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
          (items.length === 0 ? (
            <EmptyState
              imageSrc="/images/payments/payments-search.png"
              className="py-16"
              description="Такого платежа нет"
            />
          ) : (
            <div className="flex flex-col gap-6 pt-4">
              {chips.length > 0 && (
                <section aria-label="Категории">
                  <PaymentsHeading>Категории</PaymentsHeading>
                  <div className="flex flex-wrap gap-1.5 px-6 pt-2">
                    {chips.map((chip) => (
                      <ChipButton
                        key={chip.key}
                        selected={chip.selected}
                        aria-label={`Категория ${chip.label}`}
                        data-testid={`payments-search-chip-${chip.key}`}
                        onClick={() => toggleChip(chip.key)}
                      >
                        {chip.label}
                      </ChipButton>
                    ))}
                  </div>
                </section>
              )}

              <section aria-label="Платежи">
                <PaymentsHeading>Платежи</PaymentsHeading>
                <div className="flex flex-col pt-2">
                  {items.map((payment) => (
                    <PaymentRowButton
                      key={payment.id}
                      categoryIcon={<GlobalPaymentRuleIcon payment={payment} />}
                      title={payment.title}
                      subtitle={payment.propertyName}
                      subtitleIcon={
                        payment.isFavorite ? (
                          // Индикатор избранного — первый элемент второй
                          // строки (954-52461); у не-избранных слот пуст и
                          // имя встаёт на место звезды. Пассивен, управление
                          // — только со страницы платежа (решение #580).
                          <Star className="h-4 w-4" aria-hidden />
                        ) : undefined
                      }
                      amountKopecks={payment.amountKopecks}
                      description={nearestDateLine(payment)}
                      onSelect={() => openPayment(payment)}
                    />
                  ))}

                  <InfiniteQueryTail query={searchQuery} />
                </div>
              </section>
            </div>
          ))}
      </PageContent>
    </>
  );
}
