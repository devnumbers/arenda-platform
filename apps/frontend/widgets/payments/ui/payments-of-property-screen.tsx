'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import {
  usePayments,
  usePropertyOverdueOperations,
} from '@/features/payments';
import { useProperty } from '@/features/properties';
import { propertyPermissions } from '@/entities/property';
import { CategoryIcon, categoryStyle } from '@/features/payment-categories';
import {
  formatOverdueDays,
  isDatePaused,
  PaymentRowButton,
} from '@/entities/payment';
import {
  Button,
  IconButton,
  PageContent,
  StickyBottomBar,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import { clientTodayIso } from '@/entities/payment';
import type { Payment } from '@/entities/payment';
import { daysOverdue } from '../lib/overdue-days';
import { overduePaymentIdsOf, sortPaymentsByNextOccurrence } from '@/features/payments';
import { PaymentsAddSheet } from './payments-add-sheet';
import {
  PaymentRow,
  PaymentsSection,
  PaymentsSkeleton,
  PaymentsStateCard,
} from './payments-sections';

/** Лимит строк секции главного экрана (Figma 1043:57610): по 3 платежа,
 * весь список — на странице секции по клику на её заголовок. */
const SECTION_LIMIT = 3;

/**
 * Экран «Платежи объекта» (#463, Figma 1043:57610/1043:62920): три секции —
 * «Просроченные операции» (строки, красные срок и сумма), «Платежи»,
 * «Автоплатежи». Каждая секция показывает максимум 3 платежа в порядке
 * ближайшего вхождения; паузные правила на экране не выводятся (страница
 * платежа — их место), а её заголовок-стрелка ведёт на страницу секции со
 * полным списком. Закреплённая кнопка «Добавить» открывает шит выбора;
 * скрыта у смотрящего (история 47) и на архивном объекте (#446).
 *
 * Статус просрочки считает сервер (ADR 0048), подпись дней — клиентская
 * проекция (lib/overdue-days).
 */
export function PaymentsOfPropertyScreen({
  propertyId,
}: {
  readonly propertyId: string;
}): JSX.Element {
  const router = useRouter();
  const propertyQuery = useProperty(propertyId);

  const [sheetOpen, setSheetOpen] = useState(false);

  const paymentsQuery = usePayments(propertyId);
  const overdueQuery = usePropertyOverdueOperations(propertyId);

  const today = clientTodayIso();

  const payments = paymentsQuery.data ?? [];
  const overdue = overdueQuery.data ?? [];
  // Платежи с накопленной просрочкой — красная точка на иконке в секциях
  // (1323:61133, State=Expired). Просрочки приходят порцией 50 (asc) —
  // долгов поверх первой порции на плашках не бейджим.
  const overduePaymentIds = overduePaymentIdsOf(overdue);

  // Паузные правила не выводятся на экране (1043:57611) — их место на
  // странице платежа; накопленный ими долг остаётся в секции просроченных.
  const active = payments.filter((payment) => !isDatePaused(payment.pauses, today));
  const sortedRegular = sortPaymentsByNextOccurrence(
    active.filter((payment) => !payment.autoPay),
    today,
  );
  const sortedAuto = sortPaymentsByNextOccurrence(
    active.filter((payment) => payment.autoPay),
    today,
  );

  // Мутационный вход только тому, кому можно мутировать: смотрящий читает
  // без кнопок (история 47), архив read-only для финансов (#446). Пока
  // объект не загружен или не загрузился — без кнопки.
  const property = propertyQuery.isSuccess ? propertyQuery.data : undefined;
  const canMutate = propertyPermissions(property).canEdit;

  const openPayment = (payment: Payment): void =>
    router.push(ROUTES.propertyPayment(propertyId, payment.id));

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
      >
        <TopNavTitle title="Платежи объекта" />
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

          {!paymentsQuery.isPending && !overdueQuery.isPending && (
            <>
              {overdueQuery.isError ? (
                <QueryErrorCard
                  title="Не удалось загрузить просроченные"
                  onRetry={() => void overdueQuery.refetch()}
                />
              ) : (
                <PaymentsSection
                  testId="section-overdue"
                  title="Просроченные операции"
                  onOpen={() => router.push(ROUTES.propertyPaymentsOverdue(propertyId))}
                  openLabel="Открыть просроченные операции"
                  emptyHint={'У вас нет просроченных операций'}
                >
                  {overdue.length > 0
                    ? overdue.slice(0, SECTION_LIMIT).map((operation) => {
                        const style = categoryStyle('default', operation.categorySlug);
                        return (
                          <PaymentRowButton
                            key={operation.id}
                            variant="gray"
                            danger
                            categoryIcon={
                              <CategoryIcon
                                icon={style.icon}
                                color={style.color}
                                badge="danger"
                                surface="muted"
                              />
                            }
                            title={operation.title}
                            // Срок просрочки — под названием, с предлогом
                            // «на» (1043:57610), в отличие от страниц
                            // платежа, где он стоит под суммой.
                            subtitle={
                              <span className="font-medium text-danger">
                                {`на ${formatOverdueDays(daysOverdue(operation.date, today))}`}
                              </span>
                            }
                            amountKopecks={operation.amountKopecks}
                            // Плашки операций кликабельны, как на остальных
                            // поверхностях: ведут на страницу операции.
                            onSelect={() =>
                              router.push(ROUTES.propertyOperation(propertyId, operation.id))
                            }
                          />
                        );
                      })
                    : null}
                </PaymentsSection>
              )}

              {paymentsQuery.isError ? (
                <QueryErrorCard
                  title="Не удалось загрузить платежи"
                  onRetry={() => void paymentsQuery.refetch()}
                />
              ) : (
                <>
                  <PaymentsSection
                    testId="section-payments"
                    title="Платежи"
                    onOpen={() => router.push(ROUTES.propertyPaymentsAll(propertyId))}
                    openLabel="Открыть все платежи"
                    emptyHint={'Напомним, когда нужно будет отметить оплату, вы вручную отметите платеж'}
                  >
                    {sortedRegular.length > 0
                      ? sortedRegular.slice(0, SECTION_LIMIT).map((payment) => (
                          <PaymentRow
                            key={payment.id}
                            payment={payment}
                            today={today}
                            hasOverdue={overduePaymentIds.has(payment.id)}
                            onSelect={() => openPayment(payment)}
                          />
                        ))
                      : null}
                  </PaymentsSection>

                  <PaymentsSection
                    testId="section-auto"
                    title="Автоплатежи"
                    onOpen={() => router.push(ROUTES.propertyPaymentsAuto(propertyId))}
                    openLabel="Открыть автоплатежи"
                    emptyHint={'Предупредим о платеже, потом автоматически отметим оплату'}
                  >
                    {sortedAuto.length > 0
                      ? sortedAuto.slice(0, SECTION_LIMIT).map((payment) => (
                          <PaymentRow
                            key={payment.id}
                            payment={payment}
                            today={today}
                            hasOverdue={overduePaymentIds.has(payment.id)}
                            onSelect={() => openPayment(payment)}
                          />
                        ))
                      : null}
                  </PaymentsSection>
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
