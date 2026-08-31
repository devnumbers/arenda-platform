'use client';

import { useState, type JSX, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft, Cancel, Home } from '@/shared/assets/icons';
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
  CategoryIcon,
} from '@/features/payment-categories';
import { useProperty } from '@/features/properties';
import {
  isOperationPayable,
  useOperation,
  usePayOperation,
  usePaymentOperationsByStatus,
} from '@/features/payments';
import {
  Button,
  IconButton,
  PageContent,
  StatusIcon,
  StickyBottomBar,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import { PaymentsSkeleton, PaymentsStateCard } from './payments-sections';
import {
  operationDelayRow,
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
 * читают без кнопки, как на странице платежа. Заголовок корзины (удаление)
 * в этой поставке не делается.
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
  const canMutate =
    property !== undefined
    && property.access?.role !== 'viewer'
    && property.status !== 'archived';

  const loading = operationQuery.isPending || propertyQuery.isPending;
  const failed = operationQuery.isError || propertyQuery.isError;

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
      >
        {operation !== undefined && (
          <TopNavTitle title={formatDayMonth(operation.paidDate ?? operation.date)} />
        )}
      </TopNav>

      <PageContent>
        <div className="flex flex-col gap-6">
          {loading && (
            <>
              <PaymentsSkeleton withHeading />
              <PaymentsSkeleton withHeading />
            </>
          )}

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
  const router = useRouter();
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

  const today = clientTodayIso();
  const subtitle = operationSubtitle(operation, today);
  const amount = operationHeroAmount(operation);
  const delay = operationDelayRow(operation, today);
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
        {paymentId !== null && (
          <PaymentRowButton
            variant="white"
            className="px-6 py-3"
            categoryIcon={<CategoryIcon icon={category.icon} color={category.color} />}
            title={operation.title}
            subtitle="Платеж"
            onSelect={() => router.push(ROUTES.propertyPayment(propertyId, paymentId))}
          />
        )}
        <PaymentRowButton
          variant="white"
          className="px-6 py-3"
          categoryIcon={
            <span className="flex h-11 w-11 items-center justify-center rounded-pill bg-surface-muted shadow-[0_0_0_2.5px_var(--dl-surface)]">
              <Home className="h-6 w-6 text-content" aria-hidden />
            </span>
          }
          title={propertyTitle !== '' ? propertyTitle : 'Объект'}
          subtitle="Объект"
          onSelect={() => router.push(ROUTES.property(propertyId))}
        />
      </OperationSection>

      {/* Подробнее (1386:67731): статичные строки «лейбл — значение» 14px
       * прямо на белом, без карточки и без hover-подсветки. */}
      <OperationSection title="Подробнее">
        {operation.paidDate !== undefined && (
          <DetailRow label="Фактическая оплата" value={formatDayMonthWithYear(operation.paidDate, today)} />
        )}
        <DetailRow label="Плановая оплата" value={formatDayMonthWithYear(operation.date, today)} />
        {delay !== null && <DetailRow label={delay.label} value={delay.text} />}
        <DetailRow
          label="Статус"
          value={operation.status === 'overdue' ? 'Просрочена' : operation.status === 'paid' ? 'Выполнена' : 'Запланирована'}
          danger={operation.status === 'overdue'}
        />
      </OperationSection>

      {canMutate && payable && (
        <StickyBottomBar>
          <Button
            className="w-full"
            loading={payOperation.isPending}
            onClick={() => void pay()}
          >
            Отметить оплаченной
          </Button>
        </StickyBottomBar>
      )}
    </>
  );
}

/** Hero-блок (1386:67731): иконка категории 96 с белым кантом, название,
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
        className="flex h-24 w-24 items-center justify-center rounded-pill shadow-[0_0_0_2.5px_var(--dl-surface)]"
        style={{ backgroundColor: category.color }}
      >
        {Icon !== undefined && <Icon className="h-10 w-10 text-white" aria-hidden />}
      </span>
      <span className="text-xl font-semibold leading-6 text-content">{operation.title}</span>
      <span className="flex items-center gap-1.5 rounded-pill bg-surface-info py-1 pl-1.5 pr-3">
        <CategoryIcon
          icon={category.icon}
          color={category.color}
          className="h-6 w-6 [&_svg]:h-3.5 [&_svg]:w-3.5"
        />
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
              className="flex h-24 w-24 items-center justify-center rounded-pill shadow-[0_0_0_2.5px_var(--dl-surface)]"
              style={{ backgroundColor: style.color }}
            >
              {Icon !== undefined && <Icon className="h-10 w-10 text-white" aria-hidden />}
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
