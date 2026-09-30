'use client';

import { useRouter } from 'next/navigation';
import Image from 'next/image';
import { useState, type JSX, type ReactNode } from 'react';
import { Calendar, Check, Edit, Key, TrashBin } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { formatDayMonth } from '@/shared/lib/date-format';
import { formatPhoneDisplay } from '@/shared/lib/phone';
import { goBack } from '@/shared/lib/navigation';
import { notify } from '@/shared/lib/notifications';
import {
  hasProgressCard,
  rentalElapsedLine,
  rentalNextPaymentLine,
  rentalOverdueLine,
  rentalPaidTitle,
  rentalProgressPercent,
  rentalRemainingLine,
  rentalTeaserRows,
  rentalTenantTitle,
  useDeleteRental,
  type RentalActionState,
} from '@/features/rentals';
import { CategoryIcon, categoryStyle } from '@/features/payment-categories';
import type { Rental } from '@/entities/rental';
import { PaymentRowButton } from '@/entities/payment';
import { ListRow, PageContent, RoundActionButton, ConfirmDialog } from '@/shared/ui/design';
import { RentalGroup } from './rental-group';
import { TermRows } from './term-row';
import { TenantRow } from './tenant-row';

/**
 * Тело экрана «Аренда» (#531, Figma 1232:61291/1550:93664): единая для всех
 * аренд картинка-ключ 96 (решение владельца 2026-09-07, 1232:61491),
 * «Оплачено N из M месяцев», круглые действия, секции Платеж / прогресс /
 * Условия аренды / Арендатор / Управление, «Прошлые аренды» — отдельным
 * блоком под «Управлением» (1550:93664, решение #802 23.09). «Оплатить
 * платеж» ведёт на страницу операции (решение владельца 2026-09-07) —
 * оплата каноническим «Отметить оплаченной» там же, как со страницы
 * платежа. Секция «Арендатор» выводится только с арендатором. Действия
 * «Редактировать» (#532), «Продлить» (#533), «Завершить» (#534) и круглые
 * «Завершить/Продлить/Оплатить» (тройка 1550:93664, решение #802: у
 * upcoming «Завершить» скрыта) живут своими экранами.
 *
 * Статусы «ожидает начала»/«ожидает действия» макетом не нарисованы — тот
 * же рендер деградирует честно: без будущего платежа нет строки дней и
 * «Оплатить», у бессрочной нет бара — вместо остатка «Прошло N месяцев»
 * (решение владельца 2026-09-07), а карточка прогресса срочной после
 * планового окончания прячется целиком — пустого контейнера макеты не
 * рисуют (F1, решение владельца 23.09).
 *
 * Жизненные действия задаёт машина «Действий аренды» (#987, карта #984):
 * состояние едет с экрана (общий селектор с объектом, #986 — синхронность
 * поверхностей по построению). «Идёт» — «Завершить аренду» (тройка и
 * строка, мастер #534); «Ожидает начала» — завершения не существует
 * (ADR 0053 §3), его слот в тройке по-прежнему пуст (#802), а действие —
 * красная строка «Удалить аренду» (канон удаления завершённой #535: шит
 * «Удалить аренду?», успех без тоста — страница сама уходит на объект,
 * где блок «Аренда» перетекает в пустое). «Продлить» — у любой
 * незавершённой (#804), в тройку синхронизации не входит.
 */
export function RentalDetailBody({
  propertyId,
  rental,
  actionState,
  canMutate,
  hasCompletedRentals,
}: {
  readonly propertyId: string;
  readonly rental: Rental;
  /** Срез машины «Действий аренды» для текущей аренды — считается
   * экраном на полном списке (#987). */
  readonly actionState: RentalActionState;
  readonly canMutate: boolean;
  readonly hasCompletedRentals: boolean;
}): JSX.Element {
  const router = useRouter();
  const [deleteOpen, setDeleteOpen] = useState(false);

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

  // Круглая тройка макета (1550:93664, решение #802 23.09): «Завершить» —
  // только у «идёт» (машина #987: у upcoming завершения не существует,
  // слот пуст — деструктивное удаление живёт строкой «Управления»),
  // «Продлить» — у любой незавершённой (#804), раскладка по числу видимых
  // действий — тройка/пара сеткой, одиночная по центру.
  const isUpcoming = actionState.kind === 'upcoming';
  const roundCount =
    (actionState.kind === 'active' ? 1 : 0) +
    1 /* «Продлить» — у любой незавершённой (#804) */ +
    (nextPayment !== null ? 1 : 0);

  // Удаление «Ожидает начала» (#987, «передумал до старта»): DELETE
  // уносит аренду вместе с платём (бэк #985 — 204). Успех — без тоста
  // (канон #986), страница уходит на объект: аренды нет — блок «Аренда»
  // перетекает в пустое, «Управление» показывает «Начать аренду». Отказ —
  // тост канона аренды, шит остаётся открытым.
  const deleteRental = useDeleteRental(propertyId, rental.id, rentPaymentId);

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

        {canMutate &&
          roundCount > 0 && (
            /* Тройка — равными колонками (Figma 1550:93664), пара — как
                круглые кнопки страницы платежа; одиночная — по центру. */
            <div
              className={
                roundCount >= 3
                  ? 'grid grid-cols-3 justify-items-center'
                  : roundCount === 2
                    ? 'grid grid-cols-2 justify-items-center'
                    : 'flex justify-center'
              }
            >
              {/* «Завершить аренду» — левая круглая тройки (1550:93664),
                  только у «идёт» (машина #987): ведёт в мастер завершения
                  #534, как строка «Управления». Подписи тройки — в две
                  строки по макету (#802). */}
              {actionState.kind === 'active' && (
                <RoundActionButton
                  variant="secondary"
                  icon={<Key />}
                  caption={<>Завершить<br />аренду</>}
                  onClick={() => router.push(ROUTES.propertyRentalComplete(propertyId))}
                />
              )}
              <RoundActionButton
                variant="secondary"
                icon={<Calendar />}
                caption={<>Продлить<br />аренду</>}
                onClick={() => router.push(ROUTES.propertyRentalExtend(propertyId))}
              />
              {nextPayment !== null && (
                <RoundActionButton
                  variant="primary"
                  icon={<Check />}
                  caption={<>Оплатить<br />платеж</>}
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

          {/* Карточка прогресса живёт, пока ей есть что показывать; в день
              планового окончания и в «Ожидает действия» прячется целиком —
              пустого контейнера макеты не рисуют (F1, решение 23.09), но
              задолженность (#817) — содержимое: карточка снова жива. */}
          {hasProgressCard(nextPayment, rental.progress) && (
            <section className="mx-6 rounded-card bg-surface-muted p-6">
              <div className="flex flex-col gap-5">
                {/* Красная строка просрочки (#817, решение владельца 23.09):
                    первая в карточке, синяя строка дней остаётся второй. */}
                {rental.progress.overdueMonths !== null && (
                  <p className="text-sm font-medium leading-4 text-danger">
                    {rentalOverdueLine(rental.progress.overdueMonths)}
                  </p>
                )}
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
          )}

          <RentalGroup
            title="Условия аренды"
            contentGap="gap-4"
            onOpen={() => router.push(ROUTES.propertyRentalTerms(propertyId))}
            openLabel="Открыть условия аренды"
          >
            <TermRows rows={rentalTeaserRows(rental)} />
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
                phone={formatPhoneDisplay(tenant.phone)}
                onSelect={() => router.push(ROUTES.propertyContact(propertyId, tenant.contactId))}
              />
            </RentalGroup>
          )}

          {/* Секция «Управление» (Figma 1232:62297): строки ведут на экраны
              #532/#534/#533. Жизненное действие — по машине (#987): у
              «идёт» — «Завершить» (дата завершения не бывает раньше
              начала, ADR 0053 §3), у «Ожидает начала» — красное «Удалить
              аренду» (канон удаления завершённой #535). «Продлить» — у
              любой незавершённой: бессрочной задаёт первую дату окончания
              (домен «Продление», #804; как круглая кнопка #533). Смотрящему
              секции нет — все строки мутирующие. */}
          {canMutate && (
            <RentalGroup title="Управление" className="pb-3">
              <div className="flex flex-col">
                <ManageRow
                  icon={<Edit className="h-6 w-6 text-content" />}
                  label="Редактировать аренду"
                  onClick={() => router.push(ROUTES.propertyRentalTermsEdit(propertyId))}
                />
                {actionState.kind === 'active' && (
                  <ManageRow
                    icon={<Key className="h-6 w-6 text-content" />}
                    label="Завершить аренду"
                    onClick={() => router.push(ROUTES.propertyRentalComplete(propertyId))}
                  />
                )}
                {isUpcoming && (
                  <ManageRow
                    icon={<TrashBin className="h-6 w-6 text-danger" />}
                    label={<span className="text-danger">Удалить аренду</span>}
                    onClick={() => setDeleteOpen(true)}
                  />
                )}
                <ManageRow
                  icon={<Calendar className="h-6 w-6 text-content" />}
                  label="Продлить аренду"
                  onClick={() => router.push(ROUTES.propertyRentalExtend(propertyId))}
                />
              </div>
            </RentalGroup>
          )}

          {/* «Прошлые аренды» — отдельный центрированный блок под
              «Управлением» (1550:93664, решение #802 23.09): всегда у
              владельца (пустой список — честный ответ), смотрящему — только
              когда завершённые есть (чтение — и смотрящему). */}
          {(canMutate || hasCompletedRentals) && (
            <button
              type="button"
              onClick={() => router.push(ROUTES.propertyRentalPast(propertyId))}
              aria-label="Открыть прошлые аренды"
              className="mx-6 flex h-14 cursor-pointer items-center justify-center rounded-card bg-surface-muted text-base leading-[18px] text-content outline-none transition-opacity hover:opacity-90 active:opacity-90 focus-visible:ring-4 focus-visible:ring-primary"
            >
              Прошлые аренды
            </button>
          )}
        </div>
      </div>

      {/* Шит удаления «Ожидает начала» (#987): канон удаления завершённой
          аренды (#535) — канон ConfirmDialog, danger-«Удалить». Кнопка в
          loading на время мутации; успех без тоста — страница уходит на
          объект (#985 снёс аренду вместе с платём), отказ — тост, шит
          открыт. */}
      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title="Удалить аренду?"
        description="Аренда и её платеж будут удалены. Операции останутся в истории объекта."
        confirmLabel="Удалить"
        cancelLabel="Отмена"
        confirmVariant="danger"
        pending={deleteRental.isPending}
        onConfirm={() =>
          deleteRental.mutate(undefined, {
            onSuccess: () => {
              setDeleteOpen(false);
              goBack(router, ROUTES.property(propertyId));
            },
            // Канон тостов аренды (#986): ошибки удаления не молчат.
            onError: (error) => notify.scenarios.rentals.deleteError(error),
          })
        }
      />
    </PageContent>
  );
}

/** Строка секции «Управление» (Figma 1232:62297): строчная иконка 24 и
 * подпись; канонный ListRow без хвостовых слотов. Подпись — ReactNode:
 * деструктивные строки несут красный текст (канон #535). */
function ManageRow({
  icon,
  label,
  onClick,
}: {
  readonly icon: ReactNode;
  readonly label: ReactNode;
  readonly onClick: () => void;
}): JSX.Element {
  return <ListRow leading={icon} title={label} onSelect={onClick} />;
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
