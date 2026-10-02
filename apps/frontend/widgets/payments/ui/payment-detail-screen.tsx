'use client';

import { useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import {
  ArrowLeft,
  Check,
  Edit,
  Pause,
  Play,
  Repeat,
  StarOff,
  StarOutline,
} from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { OPERATIONS_FEED_SORT } from '@/shared/api/query-keys';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import { buildReturnUrl, goBack } from '@/shared/lib/navigation';
import { notify } from '@/shared/lib/notifications';
import { dateToIsoLocal } from '@/shared/lib/calendar';
import {
  formatDayMonth,
  isDatePaused,
  recurrenceLabel,
  PaymentRowButton,
  type IsoDate,
  type Payment,
  type PaymentOperation,
} from '@/entities/payment';
import {
  categoryIconComponents,
  categoryStyle,
  CategoryIcon,
} from '@/features/payment-categories';
import { useProperty } from '@/features/properties';
import { propertyPermissions } from '@/entities/property';
import {
  isPaymentCompleted,
  nearestOccurrence,
  oldestUnpaidOperation,
  paymentTypeLabel,
  usePausePayment,
  usePaymentOperationsByStatus,
  usePaymentOperationsPaged,
  usePayment,
  useResumePayment,
  useSetPaymentFavorite,
} from '@/features/payments';
import {
  Button,
  IconButton,
  Modal,
  ModalContent,
  PageContent,
  RoundActionButton,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import {
  OverdueOperationRow,
  PaymentsGroup,
  PaymentsStateCard,
} from './payments-sections';
import { paymentDetailActions } from '../lib/payment-detail-actions-model';
import { PaymentDetailSkeleton } from './payments-skeletons';

/**
 * Страница платежа (#465, макет 1096:37792/37793): шапка со звездой
 * избранного, карточка правила (цвет и иконка категории #447,
 * повторяемость; на паузе — opacity 50% и тип « • На паузе»), круглые
 * кнопки «На паузу» (confirm-шторка) ↔ «Возобновить» (без подтверждения),
 * «Изменить», «Оплатить» и три секции: «Ближайший платеж» (строка ведёт
 * на график — решение владельца #1073, у паузы тоже), «Просроченные
 * платежи» (максимум 3) и «История операций» с превью оплаченных
 * (стрелка — подэкран истории; плиток подэкранов на макете нет —
 * решение владельца #1073). Дата ближайшего вхождения — клиентская
 * проекция порта вхождений (entities/payment/lib/occurrences);
 * просрочки приходят с сервера — статус overdue считает он по TZ
 * собственника (ADR 0048).
 *
 * Копии секций — решение владельца #1073: будущие вхождения правила на
 * этой странице называются «платеж» (UI-copy exception — фиксируется в
 * payments/CONTEXT.md), «История операций» остаётся операциями.
 *
 * Доступ (ADR 0028): смотрящий — чтение, звезда неактивна, круглых кнопок
 * нет; архив финансово read-only (#446). Завершённое правило (endDate в
 * прошлом) — без паузы. «Оплатить» больше не гасит вхождение сама: она
 * ведёт на страницу операции (той же цели — старейшее неоплаченное
 * вхождение, просрочки в приоритете) с ?returnTo= этой страницы (#1072:
 * закрытие success вернёт сюда), оплата — кнопкой «Отметить
 * оплаченной» оттуда. Работает и на паузе (история 25 спеки #453).
 */

/** Максимум строк секции «Просроченные» страницы; полный список — подэкран
 * следующего среза (#466). */
const OVERDUE_PREVIEW_LIMIT = 3;

/** Колонки ряда действий по числу видимых кнопок (Tailwind-классы — только
 * литералы: динамическая строка не попала бы в CSS). */
const ACTIONS_GRID: Record<number, string> = {
  1: 'grid',
  2: 'grid grid-cols-2',
  3: 'grid grid-cols-3',
};

export function PaymentDetailScreen({
  propertyId,
  paymentId,
}: {
  readonly propertyId: string;
  readonly paymentId: string;
}): JSX.Element {
  const router = useRouter();
  const propertyQuery = useProperty(propertyId);
  const paymentQuery = usePayment(propertyId, paymentId);

  // Локали вместо сужения прямо в JSX — паттерн экрана «Платежи объекта».
  const property = propertyQuery.isSuccess ? propertyQuery.data : undefined;
  const payment = paymentQuery.isSuccess ? paymentQuery.data : undefined;

  // Единый предикат мутационного входа страницы: смотрящий читает без кнопок
  // (история 47), архив финансово read-only (#446).
  const canMutate = propertyPermissions(property).canEdit;

  const loading = paymentQuery.isPending || propertyQuery.isPending;
  const failed = paymentQuery.isError || propertyQuery.isError;

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
          payment !== undefined && (
            <FavoriteStarButton
              propertyId={propertyId}
              payment={payment}
              enabled={canMutate}
            />
          )
        }
      >
        {payment !== undefined && (
          <TopNavTitle title={payment.autoPay ? 'Автоплатёж' : 'Платеж'} />
        )}
      </TopNav>

      <PageContent>
        <div className="flex flex-col gap-8">
          {loading && <PaymentDetailSkeleton />}

          {failed && (
            <PaymentsStateCard
              title="Не удалось загрузить платеж"
              hint="Проверьте подключение и попробуйте снова"
              action={
                <Button
                  variant="secondary"
                  size="small"
                  onClick={() => {
                    void paymentQuery.refetch();
                    void propertyQuery.refetch();
                  }}
                >
                  Повторить
                </Button>
              }
            />
          )}

          {!loading && payment !== undefined && (
            <PaymentDetailBody
              propertyId={propertyId}
              payment={payment}
              canMutate={canMutate}
            />
          )}
        </div>
      </PageContent>
    </>
  );
}

/** Тело страницы после загрузки правила; просрочки и плановые операции
 * догружаются здесь — мутации секций замыкаются на их же контекст. */
function PaymentDetailBody({
  propertyId,
  payment,
  canMutate,
}: {
  readonly propertyId: string;
  readonly payment: Payment;
  readonly canMutate: boolean;
}): JSX.Element {
  const router = useRouter();
  const overdueQuery = usePaymentOperationsByStatus(propertyId, payment.id, 'overdue');
  const plannedQuery = usePaymentOperationsByStatus(propertyId, payment.id, 'planned');

  const today = dateToIsoLocal(new Date());
  const paused = isDatePaused(payment.pauses, today);
  // Завершённость для состава кнопок — клиентский предикат (история 44
  // спеки #453): серверный isCompleted у бессрочной паузы уже true.
  const completed = isPaymentCompleted(payment, today);

  // undefined — списки ещё грузятся, кнопка без состояния; null — гасить
  // нечего (завершённое правило без долга), «Оплатить» отключена.
  const payable =
    overdueQuery.data === undefined || plannedQuery.data === undefined
      ? undefined
      : oldestUnpaidOperation(overdueQuery.data, plannedQuery.data);

  const overduePreview = (overdueQuery.data ?? []).slice(0, OVERDUE_PREVIEW_LIMIT);

  return (
    <>
      {/* Карточка, кнопки и плитки — внутри 24px обёртки колонки
       * (1127:31223, padding 0 24px): секции приносят свои mx-6. */}
      <div className="px-6">
        <PaymentHeroCard payment={payment} paused={paused} />
      </div>

      {canMutate && (
        <div className="px-6">
          <PaymentActionsRow
            propertyId={propertyId}
            payment={payment}
            paused={paused}
            completed={completed}
            payable={payable}
          />
        </div>
      )}

      <div className="flex flex-col gap-6">
        <NextPaymentSection
          propertyId={propertyId}
          payment={payment}
          today={today}
          paused={paused}
          plannedOperations={plannedQuery.data ?? []}
        />

        {overdueQuery.isError ? (
          <PaymentsStateCard
            title="Не удалось загрузить просроченные"
            hint="Проверьте подключение и попробуйте снова"
            action={
              <Button
                variant="secondary"
                size="small"
                onClick={() => void overdueQuery.refetch()}
              >
                Повторить
              </Button>
            }
          />
        ) : (
          <PaymentsGroup
            title="Просроченные платежи"
            open={{
              label: 'Открыть полный список просроченных',
              onOpen: () => router.push(ROUTES.propertyPaymentOverdue(propertyId, payment.id)),
            }}
            emptyHint={overduePreview.length > 0 ? undefined : 'У вас нет просроченных платежей'}
          >
            {overduePreview.length > 0
              ? overduePreview.map((operation) => (
                  <OverdueOperationRow
                    key={operation.id}
                    operation={operation}
                    today={today}
                    onSelect={() => router.push(ROUTES.propertyOperation(propertyId, operation.id))}
                  />
                ))
              : null}
          </PaymentsGroup>
        )}

        <HistorySection propertyId={propertyId} paymentId={payment.id} />
      </div>
    </>
  );
}

/** Карточка платежа (Figma 671:6171): фон и bold-иконка — категория из
 * каталога (#447); название, сумма, направление, строка повторяемости.
 * Пауза — opacity 50% и тип « • На паузе» (история 22). */
function PaymentHeroCard({
  payment,
  paused,
}: {
  readonly payment: Payment;
  readonly paused: boolean;
}): JSX.Element {
  const style = categoryStyle(payment.category.source, payment.category.slug);
  const Icon = categoryIconComponents[style.icon];

  return (
    <section
      className={
        paused
          ? 'flex flex-col gap-3 rounded-card p-6 opacity-50 transition-opacity'
          : 'flex flex-col gap-3 rounded-card p-6 transition-opacity'
      }
      style={{ backgroundColor: style.color }}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="flex min-w-0 flex-col items-start gap-2">
          <span className="text-base leading-[18px] font-medium text-white">{payment.title}</span>
          <span className="text-xl leading-6 font-bold text-white">
            {formatMoneyKopecks(payment.amountKopecks)}
          </span>
          <span className="text-sm leading-4 font-normal text-white">
            {paymentTypeLabel(payment.type)}
            {paused && ' • На паузе'}
          </span>
        </div>
        {Icon !== undefined && <Icon className="h-16 w-16 shrink-0 text-white" aria-hidden />}
      </div>
      <div className="flex items-center gap-1.5 text-sm leading-4 font-normal text-white">
        <Repeat className="h-4 w-4 shrink-0" aria-hidden />
        <span>{recurrenceLabel(payment.recurrence)}</span>
      </div>
    </section>
  );
}

/** Круглые кнопки мутаций (Figma 671:6261): пауза с confirm-шторкой ↔
 * возобновление, «Изменить», «Оплатить» — переход на страницу операции.
 * Состав держит paymentDetailActions (#818): у управляемого арендой
 * платежа остаётся только «Оплатить». */
function PaymentActionsRow({
  propertyId,
  payment,
  paused,
  completed,
  payable,
}: {
  readonly propertyId: string;
  readonly payment: Payment;
  readonly paused: boolean;
  readonly completed: boolean;
  readonly payable: PaymentOperation | null | undefined;
}): JSX.Element {
  const router = useRouter();
  const pausePayment = usePausePayment(propertyId, payment.id);
  const resumePayment = useResumePayment(propertyId, payment.id);

  // Состав кнопок держит чистая модель (#818): управляемый арендой платёж —
  // только «Оплатить» (правка/пауза дают 409, экран их скрывает).
  const actions = paymentDetailActions(payment, paused, completed);

  const [confirmOpen, setConfirmOpen] = useState(false);

  const confirmPause = async (): Promise<void> => {
    try {
      await pausePayment.mutateAsync();
      setConfirmOpen(false);
      notify.scenarios.payments.paused();
    } catch (error) {
      notify.scenarios.payments.pauseError(error);
    }
  };

  const resume = async (): Promise<void> => {
    try {
      await resumePayment.mutateAsync();
      notify.scenarios.payments.resumed();
    } catch (error) {
      notify.scenarios.payments.resumeError(error);
    }
  };

  return (
    <>
      {/* Равные колонки по видимому составу: три кнопки, две у завершённого
       * правила (история 44), одна («Оплатить») у управляемого арендой
       * платежа (#818). */}
      <div className={ACTIONS_GRID[actions.length]}>
        {actions.includes('pause') && (
          <RoundActionButton
            icon={<Pause />}
            caption="На паузу"
            loading={pausePayment.isPending}
            onClick={() => setConfirmOpen(true)}
          />
        )}
        {actions.includes('resume') && (
          <RoundActionButton
            icon={<Play />}
            caption="Возобновить"
            loading={resumePayment.isPending}
            onClick={() => void resume()}
          />
        )}
        {/* Экран правки (#467): смотрящий не входит — строка кнопок скрыта
         * целиком (canMutate), полный доступ правит без удаления; у
         * управляемого платежа входа нет (#818). */}
        {actions.includes('edit') && (
          <RoundActionButton
            icon={<Edit />}
            caption="Изменить"
            onClick={() => router.push(ROUTES.propertyPaymentEdit(propertyId, payment.id))}
          />
        )}
        {actions.includes('pay') && (
          <RoundActionButton
            variant="primary"
            icon={<Check />}
            caption="Оплатить"
            disabled={payable == null}
            onClick={() => {
              if (payable != null) {
                // С собой — ?returnTo= этой страницы (#1072): закрытие
                // success операции вернёт сюда.
                router.push(
                  buildReturnUrl(ROUTES.propertyOperation(propertyId, payable.id), {
                    returnTo: ROUTES.propertyPayment(propertyId, payment.id),
                  }),
                );
              }
            }}
          />
        )}
      </div>

      {/* Confirm-шторка паузы (тексты — резолюция #452): случайное нажатие
       * не должно останавливать обязательство (история 21). Возобновление
       * идёт напрямую, без подтверждения. */}
      <Modal open={confirmOpen} onOpenChange={setConfirmOpen}>
        <ModalContent
          title="Поставить платеж на паузу?"
          description="Данные платежа сохранятся, вы сможете возобновить его в любое время"
        >
          <div className="mt-2 flex gap-3">
            <Button variant="secondary" className="flex-1" onClick={() => setConfirmOpen(false)}>
              Отменить
            </Button>
            <Button
              className="flex-1"
              loading={pausePayment.isPending}
              onClick={() => void confirmPause()}
            >
              Пауза
            </Button>
          </div>
        </ModalContent>
      </Modal>
    </>
  );
}

/** Секция «Ближайший платеж» (1096:37792; копии секций — решение владельца
 * #1073, заголовок без стрелки — и по макету, и по решению «стрелку
 * снести»): одна плановая дата по клиентской проекции или «На паузе»; у
 * завершённого правила вхождений больше нет — пустое состояние (история
 * 43). Клик по строке всегда открывает график — и для материализованной
 * плановой, и для проекции (решение владельца #1073): секция — вид правилу,
 * не отдельной операции. */
function NextPaymentSection({
  propertyId,
  payment,
  today,
  paused,
  plannedOperations,
}: {
  readonly propertyId: string;
  readonly payment: Payment;
  readonly today: IsoDate;
  readonly paused: boolean;
  readonly plannedOperations: ReadonlyArray<PaymentOperation>;
}): JSX.Element {
  const router = useRouter();
  // Ближайшее — серверная истина: материализованное плановое первым (после
  // оплаты тик материализует следующее, секция переезжает на него, как
  // «График»); проекция — только пока ничего не материализовано. Активная
  // пауза — «На паузе» в подзаголовке (история 37), при этом проекция не
  // считается: на паузе дат нет. Завершённое правило (isCompleted —
  // серверный вычисляемый флаг, CONTEXT.md) — пустое состояние сразу после
  // оплаты последнего вхождения, не дожидаясь endDate; иначе проекция
  // порождала бы уже оплаченные даты заново (баг «следующий платеж 31
  // августа» после полной оплаты короткого правила).
  const nearest = paused
    ? { kind: 'paused' as const }
    : payment.isCompleted
      ? undefined
      : nearestOccurrence(payment, plannedOperations, today);

  return (
    <PaymentsGroup title="Ближайший платеж">
      {nearest === undefined ? (
        // Авторский текст: состояния завершённого правила во Figma этой
        // страницы нет («Платежей еще не было» — история, не график).
        <p className="px-6 pb-4 text-sm text-content-secondary">Платеж завершен</p>
      ) : nearest.kind === 'paused' ? (
        <NextPaymentRow
          payment={payment}
          title={payment.title}
          subtitle="На паузе"
          amountKopecks={payment.amountKopecks}
          // Клик по ближайшему ведёт на график всегда (#1073, решение
          // владельца): и у паузной строки — график паузного правила
          // показывает «На паузе», путь к подэкрану не рвётся.
          onSelect={() => router.push(ROUTES.propertyPaymentSchedule(propertyId, payment.id))}
        />
      ) : (
        <NextPaymentRow
          payment={payment}
          title={nearest.kind === 'operation' ? nearest.operation.title : payment.title}
          subtitle={formatDayMonth(
            nearest.kind === 'operation' ? nearest.operation.date : nearest.date,
          )}
          amountKopecks={
            nearest.kind === 'operation' ? nearest.operation.amountKopecks : payment.amountKopecks
          }
          // Оба вида ближайшего (#1073) — клик открывает график; плановая
          // операция остаётся доступной со своих поверхностей (строки
          // просрочек, график, история).
          onSelect={() => router.push(ROUTES.propertyPaymentSchedule(propertyId, payment.id))}
        />
      )}
    </PaymentsGroup>
  );
}

function NextPaymentRow({
  payment,
  title,
  subtitle,
  amountKopecks,
  onSelect,
}: {
  readonly payment: Payment;
  readonly title: string;
  readonly subtitle: string;
  readonly amountKopecks: number;
  readonly onSelect?: () => void;
}): JSX.Element {
  const style = categoryStyle(payment.category.source, payment.category.slug);

  return (
    <PaymentRowButton
      variant="gray"
      className="px-3 pb-2"
      categoryIcon={<CategoryIcon icon={style.icon} color={style.color} surface="muted" />}
      title={title}
      subtitle={subtitle}
      amountKopecks={amountKopecks}
      onSelect={onSelect}
    />
  );
}

/** Предел превью секции «История операций»; полный список — подэкран
 * (#466). */
const HISTORY_PREVIEW_LIMIT = 3;

/** Секция «История операций» (макет 1096:37793, решение владельца #1073 —
 * плитки подэкранов снесены): стрелка ведёт в подэкран истории (#466),
 * под ней — превью оплаченных операций, новейшие сверху (порция desc по
 * фактической дате, канон истории #994). У строки — дата факта и знаковая
 * сумма (расход с минусом — канон 1302:52209), клик — страница операции.
 * Пустая история секции не имеет: без paid-операций секция не рисуется
 * (решение владельца #1073) — подэкран остаётся доступен по прямому URL,
 * путь к графику при этом держит клик по строке ближайшего. */
function HistorySection({
  propertyId,
  paymentId,
}: {
  readonly propertyId: string;
  readonly paymentId: string;
}): JSX.Element | null {
  const router = useRouter();
  // Порция desc — первые HISTORY_PREVIEW_LIMIT и есть новейшие; тот же
  // сорт, что у подэкрана (фактическая дата, OPERATIONS_FEED_SORT).
  const historyQuery = usePaymentOperationsPaged(propertyId, paymentId, {
    status: 'paid',
    order: 'desc',
    sort: OPERATIONS_FEED_SORT,
  });

  if (historyQuery.isPending || historyQuery.isError) {
    // Секция появляется после загрузки; при ошибке истории страница не
    // должна пустовать целиком — карточка состояния как у просроченных.
    return historyQuery.isError ? (
      <PaymentsStateCard
        title="Не удалось загрузить историю"
        hint="Проверьте подключение и попробуйте снова"
        action={
          <Button
            variant="secondary"
            size="small"
            onClick={() => void historyQuery.refetch()}
          >
            Повторить
          </Button>
        }
      />
    ) : null;
  }

  const preview = historyQuery.data.slice(0, HISTORY_PREVIEW_LIMIT);
  if (preview.length === 0) {
    return null;
  }

  return (
    <PaymentsGroup
      title="История операций"
      open={{
        label: 'Открыть историю операций',
        onOpen: () => router.push(ROUTES.propertyPaymentHistory(propertyId, paymentId)),
      }}
    >
      {preview.map((operation) => (
        <HistoryPreviewRow
          key={operation.id}
          operation={operation}
          onSelect={() => router.push(ROUTES.propertyOperation(propertyId, operation.id))}
        />
      ))}
    </PaymentsGroup>
  );
}

/** Строка превью истории: название, дата факта в подзаголовке (в подэкране
 * дату несёт групповой заголовок — здесь групп нет), сумма справа
 * знаковая (1302:52209). */
function HistoryPreviewRow({
  operation,
  onSelect,
}: {
  readonly operation: PaymentOperation;
  readonly onSelect?: () => void;
}): JSX.Element {
  const style = categoryStyle('default', operation.categorySlug);

  return (
    <PaymentRowButton
      variant="gray"
      className="px-3 pb-2"
      categoryIcon={<CategoryIcon icon={style.icon} color={style.color} surface="muted" />}
      title={operation.title}
      subtitle={formatDayMonth(operation.paidDate ?? operation.date)}
      amountKopecks={
        operation.type === 'expense' ? -operation.amountKopecks : operation.amountKopecks
      }
      signedAmount
      onSelect={onSelect}
    />
  );
}

/** Звезда избранного в шапке (резолюция #452): toggle для Full Access+,
 * у смотрящего видима, но неактивна. */
function FavoriteStarButton({
  propertyId,
  payment,
  enabled,
}: {
  readonly propertyId: string;
  readonly payment: Payment;
  readonly enabled: boolean;
}): JSX.Element {
  const setFavorite = useSetPaymentFavorite();

  const toggle = async (): Promise<void> => {
    try {
      await setFavorite.mutateAsync({
        propertyId,
        paymentId: payment.id,
        favorite: !payment.isFavorite,
      });
      if (!payment.isFavorite) {
        notify.scenarios.payments.favoriteAdded();
      } else {
        notify.scenarios.payments.favoriteRemoved();
      }
    } catch (error) {
      notify.scenarios.payments.favoriteError(error);
    }
  };

  return (
    <IconButton
      // 1096:37795 / 1097:40757: не в избранном — контурная звезда (клик
      // добавляет), в избранном — зачёркнутая (клик убирает).
      icon={payment.isFavorite ? <StarOff /> : <StarOutline />}
      label={payment.isFavorite ? 'Убрать из избранного' : 'Добавить в избранное'}
      aria-pressed={payment.isFavorite}
      disabled={!enabled || setFavorite.isPending}
      onClick={() => void toggle()}
    />
  );
}
