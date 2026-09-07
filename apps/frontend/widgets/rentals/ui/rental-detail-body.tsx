'use client';

import { useRouter } from 'next/navigation';
import Image from 'next/image';
import type { JSX } from 'react';
import { BoldUser, Calendar, Check } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { formatDayMonth } from '@/shared/lib/date-format';
import {
  rentalElapsedLine,
  rentalNextPaymentLine,
  rentalPaidTitle,
  rentalProgressPercent,
  rentalRemainingLine,
  rentalTeaserRows,
  rentalTenantTitle,
} from '@/features/rentals';
import { CategoryIcon, categoryStyle } from '@/features/payment-categories';
import type { Rental } from '@/entities/rental';
import { PaymentRowButton } from '@/entities/payment';
import { PageContent, RoundActionButton } from '@/shared/ui/design';
import { RentalGroup } from './rental-group';

/**
 * Тело экрана «Аренда» (#531, Figma 1232:61291/1550:93664): единая для всех
 * аренд картинка-ключ 96 (решение владельца 2026-09-07, 1232:61491),
 * «Оплачено N из M месяцев», круглые действия, секции Платеж / прогресс /
 * Условия аренды / Арендатор. «Оплатить платеж» ведёт на страницу
 * операции (решение владельца 2026-09-07) — оплата каноническим «Отметить
 * оплаченной» там же, как со страницы платежа. Секция «Арендатор» выводится
 * только с арендатором. Действия «Редактировать» (#532) и «Продлить»
 * (#533) живут своими экранами; «Завершить» (#534), кнопка «Прошлые
 * аренды» (#535) и хвостовая правка в шапке скрыты до готовности своих
 * экранов (решение владельца 2026-09-07). «Продлить аренду» — только у
 * срочной аренды: бессрочной продлевать нечего.
 *
 * Статусы «ожидает начала»/«ожидает действия» макетом не нарисованы — тот
 * же рендер деградирует честно: без будущего платежа нет строки дней и
 * «Оплатить», у бессрочной нет бара — вместо остатка «Прошло N месяцев»
 * (решение владельца 2026-09-07), у срочной после планового окончания нет
 * ни бара, ни строки остатка (фог карты #526 — сверить с владельцем на
 * приёмке).
 */
export function RentalDetailBody({
  propertyId,
  rental,
  canMutate,
}: {
  readonly propertyId: string;
  readonly rental: Rental;
  readonly canMutate: boolean;
}): JSX.Element {
  const router = useRouter();

  // Платёж арендной платы называется самим ответом аренды (связь 1:1,
  // ADR 0053 §4) — переход на экран платежа без поиска по категориям.
  const rentPaymentId = rental.rentPayment.paymentId;

  const nextPayment = rental.rentPayment.nextPayment;
  const indefinite = rental.progress.totalMonths === null;
  const percent = rentalProgressPercent(rental.progress);
  const progressLine = indefinite
    ? rentalElapsedLine(rental.startDate, rental.today)
    : rentalRemainingLine(
        rental.progress.monthsRemaining !== null && rental.progress.monthsRemaining > 0
          ? rental.progress.monthsRemaining
          : null,
      );
  const tenant = rental.tenant;
  const openPayment = () => router.push(ROUTES.propertyPayment(propertyId, rentPaymentId));

  return (
    <PageContent>
      <div className="flex flex-col gap-12">
        <div className="flex flex-col items-center gap-4">
          {/* Ключ аренды 96 (1232:61491) — одна картинка для всех аренд. */}
          <Image
            src="/images/rentals/rental-hero.png"
            alt=""
            width={96}
            height={96}
            className="h-24 w-24"
            aria-hidden
          />
          <div className="flex flex-col items-center gap-2">
            <p className="text-base leading-[18px] text-content-secondary">Оплачено</p>
            <h1 className="text-center text-[28px] font-semibold leading-8 text-content">
              {rentalPaidTitle(rental.progress)}
            </h1>
          </div>
        </div>

        {canMutate && (nextPayment !== null || rental.plannedEndDate !== null) && (
          /* Пара — равными колонками (Figma 1232:61272, как круглые кнопки
              страницы платежа); одиночная — по центру, как в #531. */
          <div
            className={
              nextPayment !== null && rental.plannedEndDate !== null
                ? 'grid grid-cols-2 justify-items-center'
                : 'flex justify-center'
            }
          >
            {/* «Завершить аренду» встанет рядом в #534. */}
            {rental.plannedEndDate !== null && (
              <RoundActionButton
                variant="secondary"
                icon={<Calendar />}
                caption="Продлить аренду"
                onClick={() => router.push(ROUTES.propertyRentalExtend(propertyId))}
              />
            )}
            {nextPayment !== null && (
              <RoundActionButton
                variant="primary"
                icon={<Check />}
                caption="Оплатить платеж"
                onClick={() =>
                  router.push(ROUTES.propertyOperation(propertyId, nextPayment.operationId))
                }
              />
            )}
          </div>
        )}

        <div className="flex flex-col gap-4">
          <RentalGroup
            title="Платеж"
            className="pb-3"
            onOpen={openPayment}
            openLabel="Открыть платеж арендной платы"
          >
            <PaymentRow rental={rental} onSelect={openPayment} />
          </RentalGroup>

          <section className="mx-6 rounded-card bg-surface-muted p-6">
            <div className="flex flex-col gap-5">
              {nextPayment !== null && (
                <div className="flex flex-col gap-3">
                  <div className="flex items-center gap-1.5">
                    <Calendar className="h-4 w-4 text-primary" aria-hidden />
                    <p className="text-sm font-medium leading-4 text-primary">
                      {rentalNextPaymentLine(nextPayment)}
                    </p>
                  </div>
                  {percent !== null && (
                    <div
                      role="progressbar"
                      aria-valuemin={0}
                      aria-valuemax={100}
                      aria-valuenow={percent}
                      className="h-1.5 w-full overflow-hidden rounded-pill bg-surface"
                    >
                      <div
                        className="h-full rounded-pill bg-primary"
                        style={{ width: `${percent}%` }}
                      />
                    </div>
                  )}
                </div>
              )}
              {progressLine !== undefined && (
                <p className="text-sm leading-4 text-content-secondary">{progressLine}</p>
              )}
            </div>
          </section>

          <RentalGroup
            title="Условия аренды"
            contentGap="gap-4"
            onOpen={() => router.push(ROUTES.propertyRentalTerms(propertyId))}
            openLabel="Открыть условия аренды"
          >
            <div className="flex flex-col gap-2 px-6">
              {rentalTeaserRows(rental).map((row) => (
                <TermRow key={row.label} label={row.label} value={row.value} />
              ))}
            </div>
          </RentalGroup>

          {tenant !== null && (
            <RentalGroup
              title="Арендатор"
              className="pb-4"
              onOpen={() => router.push(ROUTES.propertyContact(propertyId, tenant.contactId))}
              openLabel="Открыть карточку арендатора"
            >
              <TenantRow
                tenantName={rentalTenantTitle(tenant)}
                phone={tenant.phone}
                onSelect={() => router.push(ROUTES.propertyContact(propertyId, tenant.contactId))}
              />
            </RentalGroup>
          )}

          {/* Секция «Управление» (Figma 1232:62297: Редактировать аренду /
              Завершить аренду / Продлить аренду) — строки ведут на экраны
              #532/#534/#533; секция появится вместе с первым из них. */}
        </div>
      </div>
    </PageContent>
  );
}

function TermRow({ label, value }: { readonly label: string; readonly value: string }): JSX.Element {
  return (
    <div className="flex gap-3">
      <span className="shrink-0 text-sm leading-4 text-content-secondary">{label}</span>
      <span className="min-w-0 text-sm leading-4 text-content">{value}</span>
    </div>
  );
}

/** Строка платежа секции (Figma 1323:61126): иконка категории 44 с синим
 * ключом, подзаголовок — дата будущего вхождения; без него подзаголовка
 * нет (после планового окончания). */
function PaymentRow({
  rental,
  onSelect,
}: {
  readonly rental: Rental;
  readonly onSelect?: () => void;
}): JSX.Element {
  const style = categoryStyle('default', 'rent');
  return (
    <PaymentRowButton
      variant="gray"
      className="p-3"
      categoryIcon={<CategoryIcon icon={style.icon} color={style.color} surface="muted" />}
      title="Арендная плата"
      subtitle={
        rental.rentPayment.nextPayment !== null
          ? formatDayMonth(rental.rentPayment.nextPayment.date)
          : undefined
      }
      amountKopecks={rental.rentPayment.amountKopecks}
      onSelect={onSelect}
    />
  );
}

/** Строка арендатора (Figma 1232:62429): белый круг с BoldUser на серой
 * карточке, имя и телефон; «Контакта нет» — строка без действия. */
function TenantRow({
  tenantName,
  phone,
  onSelect,
}: {
  readonly tenantName: string;
  readonly phone?: string;
  readonly onSelect?: () => void;
}): JSX.Element {
  return (
    <PaymentRowButton
      variant="gray"
      className="px-3 py-2"
      categoryIcon={
        <span
          aria-hidden
          className="flex h-11 w-11 items-center justify-center rounded-pill bg-surface shadow-[0_0_0_2.5px_var(--dl-surface-muted)]"
        >
          <BoldUser className="h-6 w-6 text-content" />
        </span>
      }
      title={tenantName}
      subtitle={phone}
      onSelect={onSelect}
    />
  );
}
