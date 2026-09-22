'use client';

import { useState, type JSX, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft, BoldHome, Cancel, TrashBin } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import { goBack } from '@/shared/lib/navigation';
import { notify } from '@/shared/lib/notifications';
import {
  clientTodayIso,
  formatDayMonth,
  formatDayMonthWithYear,
  PaymentRowButton,
  type IsoDate,
  type PaymentOperation,
} from '@/entities/payment';
import {
  categoryIconComponents,
  categoryStyle,
} from '@/features/payment-categories';
import { useProperty } from '@/features/properties';
import { propertyPermissions } from '@/entities/property';
import {
  isOperationPayable,
  useDeleteOperation,
  useOperation,
  usePayOperation,
  usePaymentOperationsByStatus,
} from '@/features/payments';
import {
  Button,
  IconButton,
  Modal,
  ModalContent,
  PageContent,
  StatusIcon,
  StickyBottomBar,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import { PaymentsStateCard } from './payments-sections';
import { OperationDetailSkeleton } from './payments-skeletons';
import {
  operationDetailRows,
  operationHeroAmount,
  operationSubtitle,
} from '../lib/operation-page-model';

/**
 * Страница операции (Figma 1386:67731 / 1419:25859 / 1419:25645 /
 * 1444:66228): hero с иконкой категории, суммой и подписью срока, секции
 * «Данные операции» и «Подробнее», внизу — «Отметить оплаченной» только у
 * той операции, которую гасит «Оплатить» страницы платежа (та же
 * oldestUnpaidOperation — решение владельца). Оплата ставит paid_date =
 * сегодня сервера (ADR 0048) и показывает экран успеха «Платеж оплачен»
 * (1444:65733): «Хорошо» возвращает на страницу операции — теперь
 * «Выполнена», «Посмотреть платеж» ведёт на правило. Смотрящий и архив
 * читают без кнопок. Корзина в шапке (1386:67731) удаляет оплаченные и
 * просроченные операции через шторку 1510:77505 — tombstone cancelled
 * стирает факт и долг; плановые и проекции не удаляются.
 *
 * Manual-операция (#569/#571, макет 1858-105181): та же страница при
 * paymentId = null — в «Данных операции» только строка «Объект», кнопки
 * оплаты нет (факт рождается оплаченным), в «Подробнее» — «Дата операции»
 * и статус без строк сравнения с правилом; корзина работает как у
 * оплаченных (удаляет факт целиком).
 */
export function OperationDetailScreen({
  propertyId,
  operationId,
}: {
  readonly propertyId: string;
  readonly operationId: string;
}): JSX.Element {
  const router = useRouter();
  const operationQuery = useOperation(propertyId, operationId);
  const propertyQuery = useProperty(propertyId);

  const property = propertyQuery.isSuccess ? propertyQuery.data : undefined;
  const operation = operationQuery.isSuccess ? operationQuery.data : undefined;

  // Успех оплаты живёт на уровне экрана: в этом состоянии рисуется свой
  // экран (1444:65733) с крестиком вместо шапки «назад + дата».
  const [paidResult, setPaidResult] = useState<PaymentOperation | null>(null);


  // Единый предикат мутационного входа страницы платежа (ADR 0028, #446).
  const canMutate = propertyPermissions(property).canEdit;

  const loading = operationQuery.isPending || propertyQuery.isPending;
  const failed = operationQuery.isError || propertyQuery.isError;

  // Удаление операции (1510:77505): корзина в шапке только у оплаченных и
  // просроченных; подтверждение шторкой; после успеха — назад из истории.
  const deleteOperation = useDeleteOperation(propertyId);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const deletable =
    !loading && !failed && operation !== undefined && canMutate
    && (operation.status === 'paid' || operation.status === 'overdue');

  const remove = async (): Promise<void> => {
    if (operation === undefined) {
      return;
    }
    try {
      await deleteOperation.mutateAsync(operation.id);
      setConfirmDelete(false);
      notify.scenarios.payments.operationDeleted();
      goBack(
        router,
        operation.paymentId !== null
          ? ROUTES.propertyPayment(propertyId, operation.paymentId)
          : ROUTES.propertyPayments(propertyId),
      );
    } catch (error) {
      notify.scenarios.payments.operationDeleteError(error);
    }
  };

  if (!loading && !failed && operation !== undefined && paidResult !== null) {
    return (
      <OperationPaidSuccess
        propertyId={propertyId}
        paid={paidResult}
        propertyTitle={property?.name ?? ''}
        onClose={() => setPaidResult(null)}
      />
    );
  }

  return (
    <>
      <TopNav
        leading={
          <IconButton
            icon={<ArrowLeft />}
            label="Назад"
            onClick={() => goBack(router, ROUTES.propertyPayments(propertyId))}
          />
        }
        trailing={
          deletable && (
            <IconButton
              icon={<TrashBin />}
              label="Удалить операцию"
              onClick={() => setConfirmDelete(true)}
            />
          )
        }
      >
        {operation !== undefined && (
          <TopNavTitle title={formatDayMonth(operation.paidDate ?? operation.date)} />
        )}
      </TopNav>

      <PageContent>
        <div className="flex flex-col gap-6">
          {loading && <OperationDetailSkeleton />}

          {failed && (
            <PaymentsStateCard
              title="Не удалось загрузить операцию"
              hint="Проверьте подключение и попробуйте снова"
              action={
                <Button
                  variant="secondary"
                  size="small"
                  onClick={() => {
                    void operationQuery.refetch();
                    void propertyQuery.refetch();
                  }}
                >
                  Повторить
                </Button>
              }
            />
          )}

          {!loading && operation !== undefined && (
            <OperationDetailBody
              propertyId={propertyId}
              propertyTitle={property?.name ?? ''}
              operation={operation}
              canMutate={canMutate}
              onPaid={setPaidResult}
            />
          )}
        </div>
      </PageContent>

      {/* Confirm-шторка удаления (1510:77505): серое пояснение и мягкая
       * danger-кнопка; случайное нажатие не стирает факт (резолюция #452). */}
      <Modal open={confirmDelete} onOpenChange={setConfirmDelete}>
        <ModalContent
          title="Удалить операцию?"
          description="Операция исчезнет и не будет учитываться в доходах или расходах"
        >
          <div className="mt-2 flex gap-2">
            <Button
              variant="secondary"
              className="flex-1"
              onClick={() => setConfirmDelete(false)}
            >
              Отменить
            </Button>
            <Button
              variant="danger"
              className="flex-1"
              loading={deleteOperation.isPending}
              onClick={() => void remove()}
            >
              Удалить
            </Button>
          </div>
        </ModalContent>
      </Modal>
    </>
  );
}

/** Тело страницы после загрузки операции; списки правила для проверки
 * «которую можно оплатить» догружаются здесь же. */
function OperationDetailBody({
  propertyId,
  propertyTitle,
  operation,
  canMutate,
  onPaid,
}: {
  readonly propertyId: string;
  readonly propertyTitle: string;
  readonly operation: PaymentOperation;
  readonly canMutate: boolean;
  readonly onPaid: (paid: PaymentOperation) => void;
}): JSX.Element {
  const payOperation = usePayOperation(propertyId);

  // Ручные факты и операции удалённого правила (paymentId null) не
  // оплачиваются из этой страницы — кнопки у них и так не будет, списки
  // запроса не нужны.
  const paymentId = operation.paymentId ?? null;
  const overdueQuery = usePaymentOperationsByStatus(
    propertyId, paymentId ?? '', 'overdue', { enabled: paymentId !== null },
  );
  const plannedQuery = usePaymentOperationsByStatus(
    propertyId, paymentId ?? '', 'planned', { enabled: paymentId !== null },
  );

  const payable =
    paymentId !== null
    && overdueQuery.data !== undefined
    && plannedQuery.data !== undefined
    && isOperationPayable(operation, overdueQuery.data, plannedQuery.data);

  const pay = async (): Promise<void> => {
    try {
      onPaid(await payOperation.mutateAsync(operation.id));
    } catch (error) {
      notify.scenarios.payments.payError(error);
    }
  };

  return (
    <OperationView
      propertyId={propertyId}
      operation={operation}
      propertyTitle={propertyTitle}
      today={clientTodayIso()}
      payBar={canMutate && payable
        ? {
            onPay: () => {
              void pay();
            },
            pending: payOperation.isPending,
          }
        : undefined}
    />
  );
}

/**
 * Общий вид операции (1386:67731 / 1419:25859 / 1419:25645): hero,
 * «Данные операции» со ссылками и «Подробнее». Служит и странице
 * операции, и проекционному просмотру (без payBar — у проекции кнопки
 * нет: платится только материализованная операция).
 */
export function OperationView({
  propertyId,
  operation,
  propertyTitle,
  today,
  payBar,
}: {
  readonly propertyId: string;
  readonly operation: PaymentOperation;
  readonly propertyTitle: string;
  readonly today: IsoDate;
  readonly payBar?: { readonly onPay: () => void; readonly pending: boolean };
}): JSX.Element {
  const router = useRouter();
  const subtitle = operationSubtitle(operation, today);
  const amount = operationHeroAmount(operation);
  const details = operationDetailRows(operation, today);
  const category = categoryStyle('default', operation.categorySlug);
  const amountTone =
    amount.tone === 'success'
      ? 'text-success'
      : amount.tone === 'danger'
        ? 'text-danger'
        : 'text-content';

  return (
    <>
      <OperationHero operation={operation} category={category} subtitle={subtitle} amountText={amount.text} amountTone={amountTone} />

      {/* Данные операции (1386:67731): строки — Row Button Variant=White на
       * белом фоне, контент с отступом 24, без шеврона; клик ведёт на
       * правило и объект. У операции без правила (ручной факт, платёж
       * удалён) строки «Платеж» нет (решение владельца). */}
      <OperationSection title="Данные операции">
        {operation.paymentId !== null && (
          <PaymentRowButton
            variant="white"
            className="px-6 py-3 [&>span]:px-0"
            categoryIcon={<CategoryGlyph icon={category.icon} color={category.color} circleClass="h-11 w-11" glyphClass="h-6 w-6" />}
            title={operation.title}
            subtitle="Платеж"
            onSelect={() => router.push(ROUTES.propertyPayment(propertyId, operation.paymentId as string))}
          />
        )}
        <PaymentRowButton
          variant="white"
          className="px-6 py-3 [&>span]:px-0"
          categoryIcon={
            <span className="flex h-11 w-11 items-center justify-center rounded-pill bg-surface-muted">
              <BoldHome className="h-6 w-6 text-content-tertiary" aria-hidden />
            </span>
          }
          title={propertyTitle !== '' ? propertyTitle : 'Объект'}
          subtitle="Объект"
          onSelect={() => router.push(ROUTES.property(propertyId))}
        />
      </OperationSection>

      {/* Подробнее (1386:67731): статичные строки «лейбл — значение» 14px
       * прямо на белом, без карточки и без hover-подсветки; состав строк —
       * operationDetailRows (у manual-операции — «Дата операции» + статус,
       * 1858-105181). */}
      <OperationSection title="Подробнее">
        {details.map((row) => (
          <DetailRow
            key={row.label}
            label={row.label}
            value={row.text}
            danger={row.danger}
          />
        ))}
      </OperationSection>

      {payBar !== undefined && (
        <StickyBottomBar>
          <Button
            className="w-full"
            loading={payBar.pending}
            onClick={payBar.onPay}
          >
            Отметить оплаченной
          </Button>
        </StickyBottomBar>
      )}
    </>
  );
}

/** Hero-блок (1386:67731): иконка категории 96, название,
 * чип категории, сумма 40 и подпись срока под ней. */
function OperationHero({
  operation,
  category,
  subtitle,
  amountText,
  amountTone,
}: {
  readonly operation: PaymentOperation;
  readonly category: ReturnType<typeof categoryStyle>;
  readonly subtitle: ReturnType<typeof operationSubtitle>;
  readonly amountText: string;
  readonly amountTone: string;
}): JSX.Element {
  const Icon = categoryIconComponents[category.icon];

  return (
    <div className="flex flex-col items-center gap-3 px-6 pt-6 pb-6">
      <span
        className="flex h-24 w-24 items-center justify-center rounded-pill"
        style={{ backgroundColor: category.color }}
      >
        {Icon !== undefined && <Icon className="h-12 w-12 text-white" aria-hidden />}
      </span>
      <span className="text-xl font-semibold leading-6 text-content">{operation.title}</span>
      <span className="flex items-center gap-1.5 rounded-pill bg-surface-info py-1 pl-1.5 pr-3">
        <CategoryGlyph icon={category.icon} color={category.color} circleClass="h-6 w-6" glyphClass="h-3.5 w-3.5" />
        <span className="text-sm leading-4 text-content">{operation.categoryLabel}</span>
      </span>
      <span className={`text-[40px] leading-[44px] font-semibold ${amountTone}`}>
        {amountText}
      </span>
      {subtitle !== null && (
        <span
          className={`text-base leading-[18px] font-medium ${
            subtitle.tone === 'danger' ? 'text-danger' : 'text-primary'
          }`}
        >
          {subtitle.text}
        </span>
      )}
    </div>
  );
}

/** Секция страницы (1386:67731): заголовок H3 20/24 и строки без
 * карточки-подложки — прямо на белом фоне экрана. */
function OperationSection({
  title,
  children,
}: {
  readonly title: string;
  readonly children: ReactNode;
}): JSX.Element {
  return (
    <section>
      <h2 className="px-6 pb-2 text-xl font-semibold leading-6 text-content">{title}</h2>
      <div className="flex flex-col">{children}</div>
    </section>
  );
}

/** Круглый глиф категории без канта (правка владельца: белого кольца
 * в дизайне нет): цветная подложка заданного размера + bold-иконка. */
function CategoryGlyph({
  icon,
  color,
  circleClass,
  glyphClass,
}: {
  readonly icon: string;
  readonly color: string;
  readonly circleClass: string;
  readonly glyphClass: string;
}): JSX.Element {
  const Icon = categoryIconComponents[icon];

  return (
    <span
      className={`flex shrink-0 items-center justify-center rounded-pill ${circleClass}`}
      style={{ backgroundColor: color }}
    >
      {Icon !== undefined && <Icon className={`${glyphClass} text-white`} aria-hidden />}
    </span>
  );
}

/** Статичная строка «Подробнее»: лейбл слева серым 14, значение справа
 * тёмным 14 (просрочка — красным); hover-подсветки нет. */
function DetailRow({
  label,
  value,
  danger = false,
}: {
  readonly label: string;
  readonly value: ReactNode;
  readonly danger?: boolean;
}): JSX.Element {
  return (
    <div className="flex items-baseline justify-between gap-3 px-6 py-1 text-sm leading-4">
      <span className="shrink-0 text-content-secondary">{label}</span>
      <span className={`text-right ${danger ? 'text-danger' : 'text-content'}`}>{value}</span>
    </div>
  );
}

/** Экран успеха «Платеж оплачен» (1444:65733): иконка категории с зелёной
 * галочкой, подпись «название / сумма за дату / по объекту», кнопки
 * «Хорошо» (страница операции — теперь «Выполнена») и «Посмотреть платеж»
 * (страница правила; у ручных фактов правила нет — кнопки тоже). */
function OperationPaidSuccess({
  propertyId,
  paid,
  propertyTitle,
  onClose,
}: {
  readonly propertyId: string;
  readonly paid: PaymentOperation;
  readonly propertyTitle: string;
  readonly onClose: () => void;
}): JSX.Element {
  const router = useRouter();
  const today = clientTodayIso();
  const paidDate: IsoDate = paid.paidDate ?? today;
  const paymentId = paid.paymentId;
  const style = categoryStyle('default', paid.categorySlug);
  const Icon = categoryIconComponents[style.icon];

  return (
    <>
      <TopNav
        leading={
          <IconButton icon={<Cancel />} label="Закрыть" onClick={onClose} />
        }
      />
      <PageContent>
        <div className="flex flex-col items-center gap-8 px-6 pt-16">
          <span className="relative block h-24 w-24">
            <span
              className="flex h-24 w-24 items-center justify-center rounded-pill"
              style={{ backgroundColor: style.color }}
            >
              {Icon !== undefined && <Icon className="h-12 w-12 text-white" aria-hidden />}
            </span>
            <StatusIcon status="good" className="absolute left-[60px] top-[60px] h-12 w-12" />
          </span>
          <div className="flex flex-col gap-3 self-stretch">
            <h1 className="text-center text-xl font-semibold leading-6 text-content">
              Платеж оплачен
            </h1>
            <p className="text-center text-base leading-[18px] whitespace-pre-line text-content-secondary">
              {`«${paid.title}»\n${formatMoneyKopecks(paid.amountKopecks)} за ${formatDayMonthWithYear(paidDate, today)}\nпо объекту «${propertyTitle !== '' ? propertyTitle : '—'}»`}
            </p>
          </div>
        </div>
      </PageContent>
      <StickyBottomBar>
        <Button className="w-full" onClick={onClose}>
          Хорошо
        </Button>
        {paymentId !== null && (
          <Button
            variant="secondary"
            className="w-full"
            onClick={() => router.push(ROUTES.propertyPayment(propertyId, paymentId))}
          >
            Посмотреть платеж
          </Button>
        )}
      </StickyBottomBar>
    </>
  );
}
