'use client';

import { useState, type JSX, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import {
  ArrowLeft,
  Cancel,
  Check,
  Edit,
  EndPay,
  Key,
  Pause,
  Play,
  Repeat,
  StarOff,
  StarOutline,
} from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { OPERATIONS_FEED_SORT } from '@/shared/api/query-keys';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import { buildReturnUrl, goBack, pushPage, replacePage } from '@/shared/lib/navigation';
import { notify } from '@/shared/lib/notifications';
import { dateToIsoLocal } from '@/shared/lib/calendar';
import {
  formatDayMonthWithYear,
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
  useCompletePayment,
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
  StatusIcon,
  StickyBottomBar,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import {
  OverdueOperationRow,
  PAYMENT_HISTORY_EMPTY_HINT,
  PaymentsGroup,
  PaymentsStateCard,
} from './payments-sections';
import {
  paymentDetailActions,
  type PaymentDetailAction,
} from '../lib/payment-detail-actions-model';
import { PaymentDetailSkeleton } from './payments-skeletons';

/**
 * Страница платежа (#465, макет 3214:76417 — тикет #1194, раньше
 * 1096:37792): шапка со звездой избранного, карточка правила (цвет и
 * иконка категории #447, повторяемость; на паузе — opacity 50% и тип
 * « • На паузе»), круглые кнопки «На паузу» (confirm-шторка) ↔
 * «Возобновить» (без подтверждения), «Изменить», «Оплатить» и три секции:
 * «Ближайший платеж» (заголовок-стрелка ведёт на график — стрелка
 * возвращена макетом 3214:76417, клик строки сохранён #1073, у паузы
 * тоже), «Просроченные платежи» (максимум 3; пустое состояние — «У вас
 * нет просроченных платежей») и «История платежа» с превью оплаченных —
 * только сами операции, без плашек изменений, строки ведут на операцию
 * (копии секций — макет 3214:76417, #1194). Секция истории рисуется
 * всегда: без оплаченных — «Платежей еще не было» в карточке (решение
 * владельца #1427). Дата ближайшего вхождения —
 * клиентская проекция порта вхождений (entities/payment/lib/occurrences);
 * просрочки приходят с сервера — статус overdue считает он по TZ
 * собственника (ADR 0048).
 *
 * Завершение платежа (#1341, макеты 3249:74119/75723/75665; ревизия #1377):
 * при вычисляемой завершённости (серверный isCompleted — глоссарий
 * «Завершённый платёж») «Оплатить» заменяется на «Завершить» (EndPay) с
 * confirm-шторкой; мутация удаляет правило — успех рисует финальный экран
 * с «Хорошо» → replace списка платежей объекта (#1433; goBack выпадал на
 * шаг создания при входе «Посмотреть платеж» из успеха визарда),
 * страница ниже уже не существует (прямая ссылка — 404), финал живёт на
 * локальном состоянии и не зависит от refetch детали.
 *
 * Копии секций — макет 3214:76417 (#1194): ближайшее снова доменное
 * «Ближайшая операция» (решение #1073 «Ближайший платеж» изменено
 * макетом), история — «История платежа»; исключение остаётся только у
 * «Просроченные платежи» (payments/GLOSSARY.md).

 * Лексика и постоянный блок истории — решение владельца #1338 (12.10):
 * все заголовки страницы — «платеж»-словарь («Ближайший платеж» — макет
 * 3214:76417 держал доменное «Ближайшая операция»), блок истории стоит
 * всегда и без paid-операций несёт подзаголовок «Платежей еще не было» —
 * тот же текст, что у пустого подэкрана (пустые состояния — макеты
 * 3166:78535/1127:31705; payments/GLOSSARY.md).
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
  // правки (история 47), архив финансово read-only (#446).
  const canMutate = propertyPermissions(property).canEdit;

  // Объект в архиве (#1342): «Возобновить» задизейблена с пояснением —
  // мутации правила архивного объекта сервер отвечает 409.
  const propertyArchived = property?.status === 'archived';

  // Звезда — личное состояние читателя (D12 #1308): гейт по праву просмотра,
  // не мутации, — зритель ведёт своё избранное наравне с владельцем; архив
  // гасит (сервер отвечает 409, канон пина #1307).
  const canFavorite = propertyPermissions(property).canView && property?.status !== 'archived';

  const loading = paymentQuery.isPending || propertyQuery.isPending;
  const failed = paymentQuery.isError || propertyQuery.isError;

  // Успех завершения живёт на уровне экрана (#1341, прецедент успеха
  // оплаты #1072): в этом состоянии рисуется свой финальный экран
  // (3249:75665) с крестиком вместо шапки платежа. Правило к этому моменту
  // удалено (#1377) — финал рисуется из локального состояния независимо
  // от refetch детали (тот отвечает 404 и финал убить не должен).
  const [completedResult, setCompletedResult] = useState<Payment | null>(null);

  if (completedResult !== null) {
    return (
      <PaymentCompleteSuccess
        completed={completedResult}
        onClose={() => {
          setCompletedResult(null);
          // Детерминированный replace на список платежей объекта (решение
          // владельца #1433): goBack по истории выпадал на шаг создания,
          // когда платёж открыт «Посмотреть платеж» из экрана успеха
          // визарда — предыдущая запись истории не обязана быть списком.
          // Прецедент удаления через правку (#446, router.replace).
          replacePage(router, ROUTES.propertyPayments(propertyId));
        }}
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
          payment !== undefined && (
            <FavoriteStarButton
              propertyId={propertyId}
              payment={payment}
              enabled={canFavorite}
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
              propertyArchived={propertyArchived}
              onCompleted={setCompletedResult}
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
  propertyArchived,
  onCompleted,
}: {
  readonly propertyId: string;
  readonly payment: Payment;
  readonly canMutate: boolean;
  readonly propertyArchived: boolean;
  readonly onCompleted: (completed: Payment) => void;
}): JSX.Element {
  const router = useRouter();
  const overdueQuery = usePaymentOperationsByStatus(propertyId, payment.id, 'overdue');
  const plannedQuery = usePaymentOperationsByStatus(propertyId, payment.id, 'planned');

  const today = dateToIsoLocal(new Date());
  const paused = isDatePaused(payment.pauses, today);
  // Два уровня завершённости (глоссарий «Завершённый платёж»): endedRule —
  // клиентский предикат «расписание закончилось» (история 44 спеки #453,
  // паузы нет); settled — серверный вычисляемый флаг, им живёт замена
  // «Оплатить» → «Завершить» (#1341). Само завершение удаляет правило
  // (#1377) — хранимого read-only состояния на странице не бывает.
  const endedRule = isPaymentCompleted(payment, today);
  const settled = payment.isCompleted;

  // Состав кнопок держит модель (#818, #1341, #1342; правка владельца
  // 13.10): на архивном объекте строка не рендерится вовсе — кнопок и
  // пояснения нет, состояние несут бейдж карточки и списки; у смотрящего
  // строки тоже нет.
  const actions = paymentDetailActions(payment, paused, endedRule, settled, propertyArchived);
  const showActions = actions.length > 0 && canMutate;

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

      {showActions && (
        <div className="px-6">
          <PaymentActionsRow
            propertyId={propertyId}
            payment={payment}
            actions={actions}
            payable={payable}
            onCompleted={onCompleted}
          />
        </div>
      )}

      {/* Секции с зазором 16px (3214:76434, макет #1194 — был 24px). */}
      <div className="flex flex-col gap-4">
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
              onOpen: () => pushPage(router, ROUTES.propertyPaymentOverdue(propertyId, payment.id)),
            }}
            emptyHint={overduePreview.length > 0 ? undefined : 'У вас нет просроченных платежей'}
          >
            {overduePreview.length > 0
              ? overduePreview.map((operation) => (
                  <OverdueOperationRow
                    key={operation.id}
                    operation={operation}
                    today={today}
                    onSelect={() => pushPage(router, ROUTES.propertyOperation(propertyId, operation.id))}
                  />
                ))
              : null}
          </PaymentsGroup>
        )}

        <HistorySection propertyId={propertyId} paymentId={payment.id} today={today} />
      </div>
    </>
  );
}

/** Карточка платежа (3214:76420, макет #1194): фон и bold-иконка —
 * категория из каталога (#447); название, сумма, направление, строка
 * повторяемости (множественная форма дней месяца — recurrenceLabel).
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

/** Круглые кнопки мутаций (3249:74119, макет #1341; раньше 3214:76430 —
 * макет #1194): пауза с confirm-шторкой ↔ возобновление, «Изменить»,
 * «Оплатить» — переход на страницу операции; при вычисляемой
 * завершённости «Оплатить» заменяется на «Завершить» (иконка EndPay) с
 * confirm-шторкой (3249:75723) и финальным экраном. Состав держит
 * paymentDetailActions (#818): у управляемого арендой платежа вместо
 * «Изменить» — «Изменить аренду» (#1158), при исчерпанной оплате
 * («Ожидает действия», settled) вместо disabled «Оплатить» —
 * «Завершить аренду» (решение владельца 22.10, макет 1550:93718): та же
 * кнопка, что у тройки страницы аренды — вход в мастер завершения через
 * итоги аренды; у платежа завершённой аренды кнопок нет вовсе. */
function PaymentActionsRow({
  propertyId,
  payment,
  actions,
  payable,
  onCompleted,
}: {
  readonly propertyId: string;
  readonly payment: Payment;
  readonly actions: ReadonlyArray<PaymentDetailAction>;
  readonly payable: PaymentOperation | null | undefined;
  readonly onCompleted: (completed: Payment) => void;
}): JSX.Element {
  const router = useRouter();
  const pausePayment = usePausePayment(propertyId, payment.id);
  const resumePayment = useResumePayment(propertyId, payment.id);
  const completePayment = useCompletePayment(propertyId, payment.id);

  const [confirmOpen, setConfirmOpen] = useState(false);
  const [confirmCompleteOpen, setConfirmCompleteOpen] = useState(false);

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

  const confirmComplete = async (): Promise<void> => {
    try {
      // Ответа нет (204 — правило удалено): финал рисуется из локальной
      // сущности (#1377, прецедент успеха оплаты #1072).
      await completePayment.mutateAsync();
      setConfirmCompleteOpen(false);
      onCompleted(payment);
    } catch (error) {
      notify.scenarios.payments.completeError(error);
    }
  };

  return (
    <>
      {/* Равные колонки по видимому составу: три кнопки, две у завершённого
       * правила (история 44) и две у управляемого арендой платежа —
       * «Изменить аренду» + «Оплатить», при исчерпанной оплате —
       * «Изменить аренду» + «Завершить аренду» (решение владельца 22.10);
       * у платежа завершённой аренды ряда нет (#1158, решение 22.10). */}
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
         * управляемого платежа входа нет (#818) — вместо него «Изменить
         * аренду» (#1158), подпись в две строки, как у «Завершить аренду»
         * (решение #802): условия правятся на экране аренды, пока аренда
         * не завершена (состояние решает isRentalCompleted). */}
        {actions.includes('edit') && (
          <RoundActionButton
            icon={<Edit />}
            caption="Изменить"
            onClick={() => pushPage(router, ROUTES.propertyPaymentEdit(propertyId, payment.id))}
          />
        )}
        {actions.includes('editRental') && (
          <RoundActionButton
            icon={<Edit />}
            caption={<>Изменить<br />аренду</>}
            onClick={() => pushPage(router, ROUTES.propertyRentalTermsEdit(propertyId))}
          />
        )}
        {/* Все месяцы аренды оплачены (settled) — вместо disabled «Оплатить»
         * вход в мастер завершения аренды (решение владельца 22.10, макет
         * 1550:93718): secondary с ключом и подписью в две строки — кнопка
         * тройки страницы аренды; итоги аренды собирает сам мастер
         * (/rentals/complete). Без confirm-шторки: завершение с датой,
         * залогом и итогами — многошаговый мастер. */}
        {actions.includes('completeRental') && (
          <RoundActionButton
            variant="secondary"
            icon={<Key />}
            caption={<>Завершить<br />аренду</>}
            onClick={() => pushPage(router, ROUTES.propertyRentalComplete(propertyId))}
          />
        )}
        {actions.includes('complete') && (
          <RoundActionButton
            variant="primary"
            icon={<EndPay />}
            caption="Завершить"
            loading={completePayment.isPending}
            onClick={() => setConfirmCompleteOpen(true)}
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
                pushPage(router,
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

      {/* Confirm-шторка завершения (3249:75723, #1341): иконка категории,
       * заголовок и текст по макету; сборка канона Modal с sr-only title
       * (прецедент шита одного сеанса «Устройств» — макет ведёт иконкой).
       * Ревизия #1377: «платеж удалится» в тексте — теперь буквально
       * правда, мутация удаляет правило. */}
      <Modal open={confirmCompleteOpen} onOpenChange={setConfirmCompleteOpen}>
        <ModalContent title="Завершить платеж?" titleSrOnly>
          <CompleteSheetBody payment={payment}>
            <div className="grid grid-cols-2 gap-2">
              <Button
                variant="secondary"
                disabled={completePayment.isPending}
                onClick={() => setConfirmCompleteOpen(false)}
              >
                Отменить
              </Button>
              <Button
                loading={completePayment.isPending}
                onClick={() => void confirmComplete()}
              >
                Завершить
              </Button>
            </div>
          </CompleteSheetBody>
        </ModalContent>
      </Modal>
    </>
  );
}

/** Тело шторки завершения без кнопок (3249:75723): кружок категории 48
 * (глиф ~26), заголовок H3 и описание R/400 16/18 — вертикали макета
 * (зазоры 20 между блоками, 8 внутри); кнопки — children вызывающего. */
function CompleteSheetBody({
  payment,
  children,
}: {
  readonly payment: Payment;
  readonly children: ReactNode;
}): JSX.Element {
  const style = categoryStyle(payment.category.source, payment.category.slug);

  return (
    <div className="flex flex-col gap-5">
      <CategoryIcon
        icon={style.icon}
        color={style.color}
        className="h-12 w-12 [&>svg]:h-6.5 [&>svg]:w-6.5"
      />
      <div className="flex flex-col gap-2">
        <p className="m-0 text-xl font-semibold leading-6 text-content">Завершить платеж?</p>
        <p className="m-0 text-base leading-[18px] text-content-secondary">
          Новых операций по платежу больше не будет. После завершения, платеж удалится. Все
          операции по платежу сохранятся, их можно будет посмотреть в истории операций
        </p>
      </div>
      {children}
    </div>
  );
}

/** Секция «Ближайший платеж» (лексика — решение владельца #1338): одна
 * плановая дата по клиентской проекции или «На паузе»; при вычисляемой
 * завершённости (#1341, макет 3249:74119) секция — блок с подзаголовком
 * «Платежей больше не будет» без стрелки: графика впереди нет, вхождений
 * больше не будет. Клик по строке всегда открывает график — и для
 * материализованной плановой, и для проекции (решение владельца #1073):
 * секция — вид правилу, не отдельной операции; заголовок со стрелкой
 * ведёт туда же (3214:76436 — стрелка возвращена макетом, цель — решение
 * #1073). */
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
  // серверный вычисляемый флаг, CONTEXT.md) — блок «Платежей больше не
  // будет» сразу после оплаты последнего вхождения, не дожидаясь endDate;
  // иначе проекция порождала бы уже оплаченные даты заново (баг
  // «следующий платеж 31 августа» после полной оплаты короткого правила).
  const nearest = paused
    ? { kind: 'paused' as const }
    : payment.isCompleted
      ? undefined
      : nearestOccurrence(payment, plannedOperations, today);

  // График — цель и заголовка-стрелки (#1194), и строки (решение #1073).
  const openSchedule = (): void =>
    pushPage(router, ROUTES.propertyPaymentSchedule(propertyId, payment.id));

  return (
    <PaymentsGroup
      title="Ближайший платеж"
      // У завершённого правила стрелки нет (3249:74119, Show Arrow=false):
      // секция — состояние, не навигация.
      open={
        nearest === undefined
          ? undefined
          : { label: 'Открыть график платежей', onOpen: openSchedule }
      }
      emptyHint={nearest === undefined ? 'Платежей больше не будет' : undefined}
    >
      {/* null-дети — пустая секция PaymentsGroup: подзаголовок макета. */}
      {nearest === undefined ? null : nearest.kind === 'paused' ? (
        <NextPaymentRow
          payment={payment}
          title={payment.title}
          subtitle="На паузе"
          amountKopecks={payment.amountKopecks}
          // Клик по ближайшему ведёт на график всегда (#1073, решение
          // владельца): и у паузной строки — график паузного правила
          // показывает «На паузе», путь к подэкрану не рвётся.
          onSelect={openSchedule}
        />
      ) : (
        <NextPaymentRow
          payment={payment}
          title={nearest.kind === 'operation' ? nearest.operation.title : payment.title}
          subtitle={formatDayMonthWithYear(
            nearest.kind === 'operation' ? nearest.operation.date : nearest.date,
            today,
          )}
          amountKopecks={
            nearest.kind === 'operation' ? nearest.operation.amountKopecks : payment.amountKopecks
          }
          // Оба вида ближайшего (#1073) — клик открывает график; плановая
          // операция остаётся доступной со своих поверхностей (строки
          // просрочек, график, история).
          onSelect={openSchedule}
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
      categoryIcon={<CategoryIcon icon={style.icon} color={style.color} surface="muted" />}
      title={title}
      subtitle={subtitle}
      amountKopecks={amountKopecks}
      onSelect={onSelect}
    />
  );
}

/** Финальный экран завершения (3249:75665, #1341): крестик вместо шапки,
 * кружок категории 96 с зелёным бейджем «выполнено» (композиция канона
 * успеха — 1858:105549, как в визардах), заголовок «Платеж «Название»
 * завершен», кнопка «Хорошо» прижата к низу. Экран-состояние страницы
 * платежа: «Хорошо» и крестик — детерминированный replace списка платежей
 * объекта (#1433, решение владельца; goBack выпадал на шаг создания),
 * правило удалено (#1377), в списках ниже его уже нет, прямая ссылка
 * отвечает 404. */
function PaymentCompleteSuccess({
  completed,
  onClose,
}: {
  readonly completed: Payment;
  readonly onClose: () => void;
}): JSX.Element {
  const style = categoryStyle(completed.category.source, completed.category.slug);

  return (
    <>
      <TopNav
        leading={<IconButton icon={<Cancel />} label="Закрыть" onClick={onClose} />}
      />
      <PageContent>
        <div className="flex flex-col items-center gap-8 px-6 pt-16">
          {/* Композиция фрейма 1858:105549: кружок категории 96 (глиф ~52),
           * бейдж ~52 накладывается в угол (61, 61). */}
          <span className="relative block h-24 w-24">
            <CategoryIcon
              icon={style.icon}
              color={style.color}
              className="h-24 w-24 [&>svg]:h-13 [&>svg]:w-13"
            />
            <StatusIcon
              status="good"
              className="absolute top-[61px] left-[61px] h-13 w-13"
            />
          </span>
          <h1 className="text-center text-xl leading-6 font-semibold break-words text-content">
            {`Платеж «${completed.title}» завершен`}
          </h1>
        </div>
      </PageContent>
      <StickyBottomBar>
        <Button className="w-full" onClick={onClose}>
          Хорошо
        </Button>
      </StickyBottomBar>
    </>
  );
}

/** Предел превью секции «История платежа»; полный список — подэкран
 * (#466). */
const HISTORY_PREVIEW_LIMIT = 3;

/** Секция «История платежа» (#1194; лексика блока и подэкрана совпадает —
 * решение владельца #1338): стрелка ведёт в подэкран истории (#466), под
 * ней — превью оплаченных операций, новейшие сверху (порция desc по
 * фактической дате, канон истории #994). В превью — только сами
 * операции, без плашек изменений; у строки — дата факта и знаковая сумма
 * (расход с минусом — канон 1302:52209), клик — страница операции. Блок
 * рендерится всегда (макеты 3166:78535/1127:31705, #1338): без
 * paid-операций — серый подзаголовок «Платежей еще не было», тот же
 * текст, что у пустого подэкрана; путь к графику при этом держат
 * заголовок и строка ближайшего. */
function HistorySection({
  propertyId,
  paymentId,
  today,
}: {
  readonly propertyId: string;
  readonly paymentId: string;
  readonly today: IsoDate;
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

  return (
    <PaymentsGroup
      title="История платежа"
      open={{
        label: 'Открыть историю платежа',
        onOpen: () => pushPage(router, ROUTES.propertyPaymentHistory(propertyId, paymentId)),
      }}
      // Гейт подзаголовка внутри PaymentsGroup — children === null: без
      // paid-операций блок стоит с текстом пустого подэкрана (#1338).
      emptyHint={PAYMENT_HISTORY_EMPTY_HINT}
    >
      {preview.length > 0
        ? preview.map((operation) => (
            <HistoryPreviewRow
              key={operation.id}
              operation={operation}
              today={today}
              onSelect={() => pushPage(router, ROUTES.propertyOperation(propertyId, operation.id))}
            />
          ))
        : null}
    </PaymentsGroup>
  );
}

/** Строка превью истории: название, дата факта в подзаголовке (в подэкране
 * дату несёт групповой заголовок — здесь групп нет; год — вне текущего,
 * правило владельца 13.10), сумма справа знаковая (1302:52209). */
function HistoryPreviewRow({
  operation,
  today,
  onSelect,
}: {
  readonly operation: PaymentOperation;
  readonly today: IsoDate;
  readonly onSelect?: () => void;
}): JSX.Element {
  const style = categoryStyle('default', operation.categorySlug);

  return (
    <PaymentRowButton
      variant="gray"
      categoryIcon={<CategoryIcon icon={style.icon} color={style.color} surface="muted" />}
      title={operation.title}
      subtitle={formatDayMonthWithYear(operation.paidDate ?? operation.date, today)}
      amountKopecks={
        operation.type === 'expense' ? -operation.amountKopecks : operation.amountKopecks
      }
      signedAmount
      onSelect={onSelect}
    />
  );
}

/** Звезда избранного в шапке (резолюция #452; личный cutover #1308):
 * toggle по праву просмотра — зритель ставит наравне с владельцем и полным
 * доступом (D12); архив гасит. */
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
