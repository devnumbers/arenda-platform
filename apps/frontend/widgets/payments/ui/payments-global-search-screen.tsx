'use client';

import { useEffect, useRef, useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft, SmallArrowDown, Star } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { useSearchQueryState } from '@/shared/lib/hooks/useSearchQueryState';
import { useGlobalPaymentSearch } from '@/features/payments';
import type { GlobalPayment } from '@/entities/payment';
import { PaymentRowButton } from '@/entities/payment';
import {
  Button,
  ChipButton,
  EmptyState,
  IconButton,
  PageContent,
  SearchField,
} from '@/shared/ui/design';
import { GlobalPaymentRuleIcon, PaymentsHeading, PaymentsSkeleton, PaymentsStateCard } from './payments-sections';
import { nearestDateLine } from '../lib/payments-global-model';
import {
  effectiveChipKey,
  filterPaymentsByChip,
  searchCategoryChipKey,
  searchCategoryChips,
  searchResultRows,
} from '../lib/payments-search-model';

/**
 * Поиск платежей (#581, Figma 706:12168/12649/13008, 862:24128,
 * 860:22842) — поисковая шапка соседей (контакты #508, операции #543):
 * «назад» + поле «Поиск платежа». Старт — «Введите название платежа»,
 * пустой результат — «Такого платежа нет» (одна иллюстрация). Результаты:
 * чипы совпавших категорий (серверные matchedCategories #575; тап сужает
 * список, повторный снимает — выбранная синяя заливка) и «Платежи»
 * строками канона PaymentRowButton — как в списках карты (звезда-индикатор
 * только у избранных, пассивная — решение владельца #580); свернуто — три
 * строки и «Показать все», раскрыто — всё и «Свернуть». Тап строке —
 * страница платежа. Серверная область — GET /payments/search (#575);
 * ввод живёт в адресе (?q=) и догоняется дебаунсом — useSearchQueryState.
 * Каркас кабинетный, как поиск операций: шапка в потоке страницы, «назад»
 * возвращает на главный «Платежи».
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

  const searchQuery = useGlobalPaymentSearch(debounced, {
    enabled: debounced.trim() !== '',
  });

  const matched = searchQuery.data?.matchedCategories ?? [];
  const chips = searchCategoryChips(matched, selectedKey);
  // Выбор валиден, только пока чип есть в выдаче текущего запроса.
  const activeKey = effectiveChipKey(matched, selectedKey);
  const activeChip =
    activeKey !== null
      ? matched.find((chip) => searchCategoryChipKey(chip) === activeKey)
      : undefined;

  // Раскрытие привязано к ключу выдачи (запрос + чип): новая выдача снова
  // свернута без setState в эффекте — сравнение происходит при рендере.
  const resultKey = `${debounced}|${activeKey ?? ''}`;
  const [expandedKey, setExpandedKey] = useState<string | null>(null);
  const expanded = expandedKey === resultKey;
  const toggleExpanded = (): void =>
    setExpandedKey(expanded ? null : resultKey);

  const items = searchQuery.data?.items ?? [];
  const filtered =
    activeChip !== undefined ? filterPaymentsByChip(items, activeChip) : items;
  const { rows, hasMore } = searchResultRows(filtered, expanded);

  const emptyQuery = value.trim() === '';
  // Скелетон — только пока данных нет вовсе (первый запрос): правка запроса
  // держит прежние результаты (keepPreviousData) и не дёргает экран; ошибка
  // без данных показывает карточку повтора, не скелетон.
  const pending =
    !emptyQuery && searchQuery.data === undefined && !searchQuery.isError;
  const showResults = !emptyQuery && !pending && !searchQuery.isError;

  const toggleChip = (key: string): void =>
    setSelectedKey((current) => (current === key ? null : key));

  const openPayment = (payment: GlobalPayment): void =>
    router.push(ROUTES.propertyPayment(payment.propertyId, payment.id));

  return (
    <PageContent>
      {/* Ритм страницы — ровно 24px по бокам, как на главном «Платежи» и
       * поиске операций: шапка, секции и строки прижаты к этому краю. */}
      <div className="-mx-5 flex min-[1200px]:mx-0 flex-col gap-6 px-6 pt-1">
        <div className="flex items-center gap-1">
          <IconButton
            icon={<ArrowLeft />}
            label="Назад"
            onClick={() => goBack(router, ROUTES.payments)}
          />
          <SearchField
            ref={inputRef}
            placeholder="Поиск платежа"
            aria-label="Поиск платежа"
            value={value}
            onChange={(event) => setValue(event.target.value)}
            onClear={clear}
          />
        </div>

        {emptyQuery && (
          <EmptyState
            imageSrc="/images/payments/payments-search.png"
            className="py-16"
            description="Введите название платежа"
          />
        )}

        {!emptyQuery && searchQuery.isError && (
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
            <div className="flex flex-col gap-6">
              {chips.length > 0 && (
                <section aria-label="Категории">
                  <PaymentsHeading inset={false}>Категории</PaymentsHeading>
                  <div className="flex flex-wrap gap-1.5 pt-2">
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
                <PaymentsHeading inset={false}>Платежи</PaymentsHeading>
                <div className="flex flex-col pt-2">
                  {rows.map((payment) => (
                    <PaymentRowButton
                      key={payment.id}
                      className="-mx-3 px-0"
                      categoryIcon={<GlobalPaymentRuleIcon payment={payment} />}
                      title={payment.title}
                      subtitle={payment.propertyName}
                      subtitleSuffix={
                        payment.isFavorite ? (
                          // Индикатор избранного: пассивен, управление —
                          // только со страницы платежа (решение #580).
                          <span className="flex shrink-0" aria-hidden>
                            <Star className="h-4 w-4" />
                          </span>
                        ) : undefined
                      }
                      amountKopecks={payment.amountKopecks}
                      description={nearestDateLine(payment)}
                      onSelect={() => openPayment(payment)}
                    />
                  ))}

                  {hasMore && (
                    <Button
                      variant="secondary"
                      size="small"
                      className="mt-2 w-full"
                      data-testid="payments-search-toggle"
                      aria-expanded={expanded}
                      onClick={toggleExpanded}
                    >
                      {expanded ? 'Свернуть' : 'Показать все'}
                      <SmallArrowDown
                        className={expanded ? 'rotate-180' : undefined}
                        aria-hidden
                      />
                    </Button>
                  )}
                </div>
              </section>
            </div>
          ))}
      </div>
    </PageContent>
  );
}
