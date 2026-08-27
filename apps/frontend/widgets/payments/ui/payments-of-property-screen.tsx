'use client';

import { useEffect, useRef, useState } from 'react';
import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft, Cancel, Search } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import {
  matchesTitleSearch,
  usePayments,
  usePropertyOverdueOperations,
} from '@/features/payments';
import { useProperty } from '@/features/properties';
import { CategoryIcon, categoryStyle } from '@/features/payment-categories';
import { formatOverdueDays, PaymentCardButton } from '@/entities/payment';
import {
  Button,
  IconButton,
  PageContent,
  SearchField,
  StickyBottomBar,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import { clientTodayIso } from '@/entities/payment';
import { daysOverdue } from '../lib/overdue-days';
import { PaymentsAddSheet } from './payments-add-sheet';
import {
  PaymentsEmptyCard,
  PaymentsGroup,
  PaymentsHeading,
  PaymentsSkeleton,
  PaymentsStateCard,
  PaymentRow,
} from './payments-sections';

/**
 * Экран «Платежи объекта» (#463, Figma 784:13393 / 654:6778, 853:17208):
 * секции «Просроченные» (карточки, горизонтальный скролл), «Платежи»,
 * «Автоплатежи», поиск по названиям и закреплённая кнопка «Добавить»,
 * открывающая шит выбора. Данные — список платежей объекта и просроченные
 * операции объекта; статус просрочки считает сервер (ADR 0048), подпись
 * дней — клиентская проекция (lib/overdue-days).
 *
 * Кнопка «Добавить» — вход в финансовую мутацию: скрыта у смотрящего
 * (история 47) и на архивном объекте (read-only архива, #446). Стрелки-ссылки
 * заголовков секций из Figma не рисуются: адресаты (полный список
 * просроченных, глобальные списки) — следующие срезы, мёртвых ссылок не
 * выпускаем. Строки и карточки открывают страницу платежа (#465). Текст
 * карточки «Ничего не нашлось» — авторский: состояния поиска в Figma нет.
 */
export function PaymentsOfPropertyScreen({
  propertyId,
}: {
  readonly propertyId: string;
}): JSX.Element {
  const router = useRouter();
  const propertyQuery = useProperty(propertyId);
  const paymentsQuery = usePayments(propertyId);
  const overdueQuery = usePropertyOverdueOperations(propertyId);

  const [searchOpen, setSearchOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [sheetOpen, setSheetOpen] = useState(false);
  const searchInputRef = useRef<HTMLInputElement>(null);

  // Поле поиска получает фокус программно (jsx-a11y запрещает autoFocus;
  // паттерн логин-экрана — ref + effect под контролем компонента).
  useEffect(() => {
    if (searchOpen) {
      searchInputRef.current?.focus();
    }
  }, [searchOpen]);

  const today = clientTodayIso();

  const payments = paymentsQuery.data ?? [];
  const overdue = overdueQuery.data ?? [];

  const searching = searchOpen && query.trim().length > 0;
  const regularPayments = payments.filter((payment) => !payment.autoPay);
  const autoPayments = payments.filter((payment) => payment.autoPay);

  const visibleRegular = searching
    ? regularPayments.filter((payment) => matchesTitleSearch(query, payment.title))
    : regularPayments;
  const visibleAuto = searching
    ? autoPayments.filter((payment) => matchesTitleSearch(query, payment.title))
    : autoPayments;
  const visibleOverdue = searching
    ? overdue.filter((operation) => matchesTitleSearch(query, operation.title))
    : overdue;

  const searchMissed =
    searching
    && visibleOverdue.length === 0
    && visibleRegular.length === 0
    && visibleAuto.length === 0;

  // Мутационный вход только тому, кому можно мутировать: смотрящий читает
  // без кнопок (история 47), архив read-only для финансов (#446). Пока
  // объект не загружен или не загрузился — без кнопки.
  const property = propertyQuery.isSuccess ? propertyQuery.data : undefined;
  const role = property?.access?.role;
  const canMutate =
    property !== undefined && role !== undefined && role !== 'viewer' && property.status !== 'archived';

  const closeSearch = (): void => {
    setSearchOpen(false);
    setQuery('');
  };

  return (
    <>
      <TopNav
        leading={
          <IconButton
            icon={<ArrowLeft />}
            label="Назад"
            onClick={() => goBack(router, ROUTES.property(propertyId))}
          />
        }
        trailing={
          searchOpen ? (
            <IconButton icon={<Cancel />} label="Закрыть поиск" onClick={closeSearch} />
          ) : (
            <IconButton
              icon={<Search />}
              label="Поиск"
              onClick={() => setSearchOpen(true)}
            />
          )
        }
      >
        {searchOpen ? (
          <SearchField
            ref={searchInputRef}
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            onClear={() => setQuery('')}
            placeholder="Найти платеж"
            aria-label="Поиск по названиям"
          />
        ) : (
          <TopNavTitle title="Платежи объекта" />
        )}
      </TopNav>

      <PageContent>
        <div className="flex flex-col gap-4">
          {(paymentsQuery.isPending || overdueQuery.isPending) && (
            <>
              <PaymentsSkeleton withHeading />
              <PaymentsSkeleton withHeading />
              <PaymentsSkeleton withHeading />
            </>
          )}

          {paymentsQuery.isError && (
            <QueryErrorCard
              title="Не удалось загрузить платежи"
              onRetry={() => void paymentsQuery.refetch()}
            />
          )}

          {!paymentsQuery.isPending && !paymentsQuery.isError && (
            <>
              {searchMissed ? (
                <PaymentsEmptyCard
                  title="Ничего не нашлось"
                  hint="Попробуйте изменить поисковый запрос"
                />
              ) : (
                <>
                  {overdueQuery.isError ? (
                    <QueryErrorCard
                      title="Не удалось загрузить просроченные"
                      onRetry={() => void overdueQuery.refetch()}
                    />
                  ) : visibleOverdue.length > 0 ? (
                    <section className="flex flex-col">
                      <PaymentsHeading>Просроченные</PaymentsHeading>
                      <div className="mt-3 flex gap-2 overflow-x-auto px-6 pb-1">
                        {visibleOverdue.map((operation) => {
                          const style = categoryStyle('default', operation.categorySlug);
                          const paymentId = operation.paymentId;
                          return (
                            <PaymentCardButton
                              key={operation.id}
                              title={operation.title}
                              amountKopecks={operation.amountKopecks}
                              description={formatOverdueDays(daysOverdue(operation.date, today))}
                              leading={
                                <CategoryIcon
                                  icon={style.icon}
                                  color={style.color}
                                  badge="danger"
                                  surface="muted"
                                />
                              }
                              danger
                              onSelect={
                                paymentId !== null
                                  ? () => router.push(ROUTES.propertyPayment(propertyId, paymentId))
                                  : undefined
                              }
                            />
                          );
                        })}
                      </div>
                    </section>
                  ) : (
                    !searching && (
                      <PaymentsEmptyCard
                        title="Нет просроченных платежей"
                        hint="Когда платеж просрочится, он будет здесь"
                      />
                    )
                  )}

                  <div className="flex flex-col gap-4">
                    {visibleRegular.length > 0 ? (
                      <PaymentsGroup title="Платежи">
                        {visibleRegular.map((payment) => (
                          <PaymentRow
                            key={payment.id}
                            payment={payment}
                            today={today}
                            onSelect={() =>
                              router.push(ROUTES.propertyPayment(propertyId, payment.id))
                            }
                          />
                        ))}
                      </PaymentsGroup>
                    ) : (
                      !searching && (
                        <PaymentsEmptyCard
                          title="Нет платежей"
                          hint="Напомним, когда нужно будет отметить оплату, вы вручную отметите платеж"
                        />
                      )
                    )}

                    {visibleAuto.length > 0 ? (
                      <PaymentsGroup title="Автоплатежи">
                        {visibleAuto.map((payment) => (
                          <PaymentRow
                            key={payment.id}
                            payment={payment}
                            today={today}
                            onSelect={() =>
                              router.push(ROUTES.propertyPayment(propertyId, payment.id))
                            }
                          />
                        ))}
                      </PaymentsGroup>
                    ) : (
                      !searching && (
                        <PaymentsEmptyCard
                          title="Нет автоплатежей"
                          hint="Предупредим о платеже, потом автоматически отметим оплату"
                        />
                      )
                    )}
                  </div>
                </>
              )}
            </>
          )}
        </div>
      </PageContent>

      {canMutate && (
        <StickyBottomBar>
          <Button className="w-full" onClick={() => setSheetOpen(true)}>
            Добавить
          </Button>
        </StickyBottomBar>
      )}

      <PaymentsAddSheet propertyId={propertyId} open={sheetOpen} onOpenChange={setSheetOpen} />
    </>
  );
}

/** Карточка ошибки запроса с повтором — общий вид для обоих списков экрана. */
function QueryErrorCard({
  title,
  onRetry,
}: {
  readonly title: string;
  readonly onRetry: () => void;
}): JSX.Element {
  return (
    <PaymentsStateCard
      title={title}
      hint="Проверьте подключение и попробуйте еще раз"
      action={
        <Button size="small" variant="secondary" onClick={onRetry}>
          Повторить
        </Button>
      }
    />
  );
}

