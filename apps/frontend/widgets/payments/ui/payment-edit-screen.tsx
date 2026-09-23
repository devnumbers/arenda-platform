'use client';

import { useEffect, useRef, useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import {
  ArrowLeft,
  Calendar,
  Cancel,
  ChangeVertical,
  Check,
  Filter,
  Search,
  TrashBin,
} from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import {
  kopecksToRublesString,
  parseRublesToKopecks,
} from '@/shared/lib/format-money';
import { goBack } from '@/shared/lib/navigation';
import { notify } from '@/shared/lib/notifications';
import {
  clientTodayIso,
  formatDayMonthWithYear,
  recurrenceLabel,
  type IsoDate,
  type Payment,
  type Recurrence,
} from '@/entities/payment';
import { paymentCategoryBySlug } from '@/features/payment-categories';
import { propertyPermissions } from '@/entities/property';
import { useProperty } from '@/features/properties';
import {
  buildPaymentUpdateCommand,
  branchKind,
  editFormReady,
  FORM_OF_PAYMENT_LABELS,
  periodicityReady,
  recurrencesEqual,
  togglePaymentForm,
  togglePaymentType,
  TYPE_LABELS,
  useDeletePayment,
  usePayment,
  usePaymentOperationsByStatus,
  useUpdatePayment,
  type PaymentEditForm,
  type PeriodicityBranch,
} from '@/features/payments';
import {
  Button,
  CalendarDatePicker,
  Checkbox,
  groupedAmount,
  IconButton,
  Modal,
  ModalContent,
  PageContent,
  sanitizeAmountInput,
  SearchField,
  StickyBottomBar,
  syncAmountInputDom,
  TextField,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import {
  PaymentsEmptyCard,
  PaymentsStateCard,
} from './payments-sections';
import { PaymentEditFormSkeleton } from './payments-skeletons';
import { CategoryStep } from './payment-create-wizard/category-step';
import {
  BRANCH_PERIOD_LABELS,
  PeriodicityStep,
} from './payment-create-wizard/periodicity-step';
import { CategorySearchHint } from './payment-create-wizard/wizard-chrome';

/**
 * Экран правки платежа (Figma 705:10034; ранее 1127:33146, #467): форма,
 * не визард — все поля на одной странице, значения предзаполнены правилом.
 * В хедере — только «Редактирование» (решение владельца 2026-08-31).
 * Тип («Доход или расход») и «Способ оплаты» — строки-поля, значение
 * меняется простым нажатием по строке, без пикеров. Лейбл «Способ оплаты» —
 * по макету (решение владельца 2026-08-31), хотя глоссарий (CONTEXT-MAP)
 * считает его термином биллинга и предписывает платежам «Форму оплаты».
 * Категория — отдельная страница на том же маршруте, как шаг 1 визарда
 * (Figma 781:12299, без карандаша: свои категории — следующий срез);
 * заголовок хедера — «Выбор категории», заголовок страницы без подписи
 * (решение владельца 2026-08-31); уход со страницы не теряет
 * несохранённые правки формы. Периодичность — отдельная страница как шаг 3
 * визарда, хедер «Выбор периодичности», в ветке — название периода
 * (решение владельца 2026-08-31); правка живёт в черновике страницы и
 * применяется только кнопкой «Выбрать» — «Назад» её отбрасывает
 * (багфикс: незавершённый период не оставался в форме). Окончание —
 * канонический бесконечный календарь поверх формы (решение владельца
 * 2026-09-04; раньше — страница с календарём, открытым сразу,
 * решение 2026-08-31). Регулярность —
 * без «Один раз» (решение #449) с ветками дат; `since` и напоминания
 * отсутствуют (не редактируются, истории 31 спеки #453).
 * Сохранение — частичный PATCH: команда — дифф формы (update-model),
 * пересоздание планового вхождения делает сервер — правка меняет только
 * будущее. Внизу — danger-кнопка «Удалить платеж» с модалкой выбора судьбы
 * просрочек (тексты — фрейм 1127:33148): без чекбокса долг остаётся
 * (`keep_overdue=true`), с чекбоксом — сносится; после удаления — тост на
 * списке платежей объекта.
 *
 * Доступ (ADR 0028): смотрящий экрана не видит вовсе; полный доступ правит
 * без удаления; «Удалить платеж» — только владелец. Архив финансово
 * read-only (#446). Платёж, управляемый арендой (#818), деградирует к
 * карточке «Правка недоступна» с арендной подсказкой — условия задаются
 * в аренде.
 */

export function PaymentEditScreen({
  propertyId,
  paymentId,
}: {
  readonly propertyId: string;
  readonly paymentId: string;
}): JSX.Element {
  const router = useRouter();
  const propertyQuery = useProperty(propertyId);
  const paymentQuery = usePayment(propertyId, paymentId);

  const property = propertyQuery.isSuccess ? propertyQuery.data : undefined;
  const payment = paymentQuery.isSuccess ? paymentQuery.data : undefined;

  const canEdit = propertyPermissions(property).canEdit;
  // Управляемый арендой платёж правится только через аренду (#818): экран
  // деградирует как у смотрящего, но с арендной подсказкой — удаления у
  // такого платежа не существует вовсе (RESTRICT FK, честный 409).
  const managed = payment?.isRentalManaged === true;
  const editable = canEdit && !managed;
  // Удаление — только владелец (история 49 спеки #453).
  const canDelete = property?.access?.role === 'owner';

  const loading = propertyQuery.isPending || paymentQuery.isPending;
  const failed = propertyQuery.isError || paymentQuery.isError;

  return (
    <>
      {/* В загрузке роль неизвестна — показываем хром правки (сценарий
       * по умолчанию); у смотрящего, архива и управляемого арендой
       * платежа после загрузки хром деградирует к «Отмене», а контент —
       * к карточке недоступности (редкий прямой путь, решение #607). */}
      {!loading && !editable && (
        <TopNav
          leading={
            <IconButton
              icon={<Cancel />}
              label="Отменить правку"
              onClick={() => goBack(router, ROUTES.propertyPayment(propertyId, paymentId))}
            />
          }
        />
      )}

      <PageContent>
        {/* Хедер правки стабилен во всех фазах (#607): и в загрузке, и в
         * форме EditingHeader стоит первым ребёнком контейнера — шапка не
         * меняет позицию, когда данные пришли. */}
        <div className="flex flex-col gap-8">
          {loading && (
            <>
              <EditingHeader
                checkDisabled
                onCancel={() => goBack(router, ROUTES.propertyPayment(propertyId, paymentId))}
                onSave={() => {}}
              />
              <PaymentEditFormSkeleton />
            </>
          )}

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

          {!loading && !failed && !editable && (
            <div className="pt-6">
              <PaymentsEmptyCard
                title="Правка недоступна"
                hint={
                  managed
                    ? 'Платёж управляется арендой — изменить его можно только в аренде'
                    : property?.status === 'archived'
                      ? 'Объект в архиве — платежи можно только смотреть'
                      : 'У вас доступ только для просмотра этого объекта'
                }
              />
            </div>
          )}

          {!loading && !failed && editable && payment !== undefined && (
            <PaymentEditForm
              key={payment.id}
              propertyId={propertyId}
              payment={payment}
              canDelete={canDelete}
            />
          )}
        </div>
      </PageContent>

      {/* Sticky-панель — постоянная часть экрана: в загрузке та же кнопка
       * в покое (#607), форма подменяет её без сдвига. */}
      {loading && (
        <StickyBottomBar>
          <Button className="w-full" disabled>
            Сохранить изменения
          </Button>
        </StickyBottomBar>
      )}
    </>
  );
}

/** Шапка экрана правки (Figma 705:10034): Отмена | «Редактирование» |
 * Check — быстрая клавиша сохранения наравне со sticky-кнопкой. Общая
 * форме и фазе загрузки — хедер не меняется, когда данные пришли. */
function EditingHeader({
  checkDisabled,
  onCancel,
  onSave,
}: {
  readonly checkDisabled: boolean;
  readonly onCancel: () => void;
  readonly onSave: () => void;
}): JSX.Element {
  return (
    <TopNav
      leading={
        <IconButton icon={<Cancel />} label="Отменить правку" onClick={onCancel} />
      }
      trailing={
        <IconButton
          icon={<Check />}
          label="Сохранить"
          disabled={checkDisabled}
          onClick={onSave}
        />
      }
    >
      <TopNavTitle title="Редактирование" />
    </TopNav>
  );
}

/** Заголовок формы: значение предзаполнено правилом, пустой ввод заменяет
 * лейбл категории при сохранении (правила визарда #464). */
function formFromPayment(payment: Payment): PaymentEditForm {
  return {
    type: payment.type,
    title: payment.title,
    amountKopecks: payment.amountKopecks,
    categorySlug: undefined,
    paymentForm: payment.paymentForm,
    recurrence: payment.recurrence,
    endDate: payment.endDate,
  };
}

/** Сумма в поле — как набирает пользователь, без хвостовых нулей
 * («3200», «1500,5»); обратный разбор — только через canonical-модуль денег. */
function initialAmountRaw(amountKopecks: number): string {
  const raw = kopecksToRublesString(amountKopecks).replace('.', ',');
  return raw.endsWith(',00') ? raw.slice(0, -3) : raw.replace(/0$/, '');
}

function PaymentEditForm({
  propertyId,
  payment,
  canDelete,
}: {
  readonly propertyId: string;
  readonly payment: Payment;
  /** Удаление — только владелец; у полного доступа кнопки и модалки нет. */
  readonly canDelete: boolean;
}): JSX.Element {
  const router = useRouter();
  const updatePayment = useUpdatePayment(propertyId, payment.id);
  const deletePayment = useDeletePayment(propertyId, payment.id);

  const today: IsoDate = clientTodayIso();
  const resolveTitle = (slug: string): string | undefined =>
    paymentCategoryBySlug(slug)?.label;

  const [form, setForm] = useState<PaymentEditForm>(() => formFromPayment(payment));
  const [amountRaw, setAmountRaw] = useState<string>(() => initialAmountRaw(payment.amountKopecks));

  const [endDateOpen, setEndDateOpen] = useState(false);
  const [openBranch, setOpenBranch] = useState<PeriodicityBranch | null>(null);
  // Периодичность — отдельная страница на том же маршруте (как шаг 3
  // визарда); правка живёт в черновике страницы и попадает в форму только
  // по кнопке «Выбрать» — иначе незавершённый период («Каждую неделю в —»)
  // оставался бы в форме при выходе назад.
  const [periodicityOpen, setPeriodicityOpen] = useState(false);
  const [periodicityDraft, setPeriodicityDraft] = useState<Recurrence | undefined>(
    undefined,
  );

  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deleteOverdue, setDeleteOverdue] = useState(false);

  // Выбор категории — отдельная страница на том же маршруте (как шаг 1
  // визарда): уход со страницы не теряет несохранённые правки формы.
  const [categoryOpen, setCategoryOpen] = useState(false);
  const [categorySearchOpen, setCategorySearchOpen] = useState(false);
  const [categoryQuery, setCategoryQuery] = useState('');
  const searchInputRef = useRef<HTMLInputElement | null>(null);

  // Открытие поиска сразу делает поле активным (программный фокус —
  // устоявшийся a11y-паттерн вместо autoFocus).
  useEffect(() => {
    if (categorySearchOpen) {
      searchInputRef.current?.focus();
    }
  }, [categorySearchOpen]);

  // Клик вне хедера при пустом запросе закрывает поиск (Figma 1049:46256);
  // с непустым поиск остаётся открытым.
  useEffect(() => {
    if (!categorySearchOpen || categoryQuery !== '') {
      return undefined;
    }
    const handleOutside = (event: MouseEvent): void => {
      if (!(event.target instanceof Element) || !event.target.closest('header')) {
        setCategorySearchOpen(false);
      }
    };
    document.addEventListener('mousedown', handleOutside);
    return () => document.removeEventListener('mousedown', handleOutside);
  }, [categorySearchOpen, categoryQuery]);

  // Чекбокс модалки удаления виден ровно при наличии просрочек (история 35) —
  // список тянут только те, кому удаление доступно.
  const overdueQuery = usePaymentOperationsByStatus(propertyId, payment.id, 'overdue', {
    enabled: canDelete,
  });
  const overdueCount = overdueQuery.data?.length ?? 0;

  const update =
    <K extends keyof PaymentEditForm>(key: K, value: PaymentEditForm[K]): void =>
    setForm((prev) => ({ ...prev, [key]: value }));

  const categoryLabel =
    form.categorySlug !== undefined
      ? resolveTitle(form.categorySlug) ?? form.categorySlug
      : payment.category.label;

  // Выбор на странице категории: явный выбор формы или текущая категория.
  const selectedSlug = form.categorySlug ?? payment.category.slug;
  const categoryChanged =
    form.categorySlug !== undefined && form.categorySlug !== payment.category.slug;

  const command = buildPaymentUpdateCommand(payment, form, { resolveTitle });
  const ready = editFormReady(form, payment.category.slug, resolveTitle);
  const canSave = ready && command !== undefined;

  const save = async (): Promise<void> => {
    if (command === undefined) {
      return;
    }
    try {
      await updatePayment.mutateAsync(command);
      notify.scenarios.payments.updated();
      goBack(router, ROUTES.propertyPayment(propertyId, payment.id));
    } catch (error) {
      notify.scenarios.payments.updateError(error);
    }
  };

  const confirmDelete = async (): Promise<void> => {
    try {
      // Чекбокс не отмечен — просрочки остаются долгом (безопасный дефолт).
      const keepOverdue = !deleteOverdue;
      await deletePayment.mutateAsync(keepOverdue);
      setDeleteOpen(false);
      // Тост живёт в глобальном портале — виден уже на списке объекта;
      // replace: экран правки удалённого правила из истории уходит.
      notify.scenarios.payments.deleted();
      router.replace(ROUTES.propertyPayments(propertyId));
    } catch (error) {
      notify.scenarios.payments.deleteError(error);
    }
  };

  const closePeriodicityPage = (): void => {
    // Черновик отбрасывается: период меняется только кнопкой «Выбрать».
    setPeriodicityOpen(false);
    setOpenBranch(null);
    setPeriodicityDraft(undefined);
  };

  const applyPeriodicity = (): void => {
    if (periodicityDraft === undefined) {
      return;
    }
    update('recurrence', periodicityDraft);
    closePeriodicityPage();
  };

  const periodicityBack = (): void => {
    if (openBranch !== null) {
      setOpenBranch(null);
      return;
    }
    closePeriodicityPage();
  };

  // Окончание платежа — канонический бесконечный календарь поверх формы
  // (как выбор даты в задачах; решение владельца 2026-09-04, раньше была
  // страница с календарём, открытым сразу): «Выбрать» применяет дату,
  // снятая (null) — бессрочно (в команде PATCH — tri-state endDate: null).
  // Прошлое закрыто, задним числом дата не назначается.

  const closeCategoryPage = (): void => {
    setCategoryOpen(false);
    setCategorySearchOpen(false);
    setCategoryQuery('');
  };

  // Страница выбора категории (Figma 781:12299): радио на строке — состояние
  // формы, «Готово» появляется после смены и возвращает на форму; «Назад»
  // тоже возвращает — выбор уже в форме, его видно и можно не сохранять.
  if (categoryOpen) {
    return (
      <>
        <TopNav
          leading={
            <IconButton icon={<ArrowLeft />} label="Назад" onClick={closeCategoryPage} />
          }
          trailing={
            categorySearchOpen ? undefined : (
              <IconButton
                icon={<Search />}
                label="Поиск по категориям"
                onClick={() => setCategorySearchOpen(true)}
              />
            )
          }
        >
          {categorySearchOpen ? (
            <SearchField
              ref={searchInputRef}
              value={categoryQuery}
              onChange={(event) => setCategoryQuery(event.target.value)}
              onClear={() => {
                setCategoryQuery('');
                setCategorySearchOpen(false);
              }}
              placeholder="Найти категорию"
              aria-label="Поиск по названиям категорий"
            />
          ) : (
            <TopNavTitle title="Выбор категории" />
          )}
        </TopNav>
        {/* КатегорияStep и подсказка поиска приносят свои отступы (шаг визарда
         * рассчитан на полноширинный контент) — обёртке паддинг не нужен. */}
        {categorySearchOpen && categoryQuery === '' ? (
          <CategorySearchHint text="Начните искать категорию" />
        ) : (
          <CategoryStep
            selectedSlug={selectedSlug}
            onSelect={(slug) => update('categorySlug', slug)}
            query={categorySearchOpen ? categoryQuery : ''}
          />
        )}
        {categoryChanged && (
          <StickyBottomBar>
            <Button className="w-full" onClick={closeCategoryPage}>
              Готово
            </Button>
          </StickyBottomBar>
        )}
      </>
    );
  }

  // Страница периодичности — как шаг 3 визарда (Figma 1049:48174): в ветке
  // хедер показывает название периода, «Назад» закрывает ветку; правка — в
  // черновике, «Выбрать» применяется когда периодичность готова, ветка
  // закрыта или совпадает с ней и значение изменилось.
  if (periodicityOpen) {
    const draftChanged =
      periodicityDraft !== undefined
      && !recurrencesEqual(periodicityDraft, form.recurrence);
    return (
      <>
        <TopNav
          leading={
            <IconButton icon={<ArrowLeft />} label="Назад" onClick={periodicityBack} />
          }
        >
          {openBranch !== null ? (
            <TopNavTitle title={BRANCH_PERIOD_LABELS[openBranch]} />
          ) : (
            <TopNavTitle title="Выбор периодичности" />
          )}
        </TopNav>
        <PeriodicityStep
          recurrence={periodicityDraft}
          openBranch={openBranch}
          onOpenBranch={setOpenBranch}
          onRecurrenceChange={setPeriodicityDraft}
          onDailyPick={() => setPeriodicityDraft({ kind: 'daily' })}
          today={today}
          withHeading={false}
        />
        {periodicityDraft !== undefined
          && periodicityReady(periodicityDraft)
          && (openBranch === null || openBranch === branchKind(periodicityDraft))
          && draftChanged && (
          <StickyBottomBar>
            <Button className="w-full" onClick={applyPeriodicity}>
              Выбрать
            </Button>
          </StickyBottomBar>
        )}
      </>
    );
  }

  return (
    <>
      {/* Шапка EditingHeader (Figma 705:10034): Отмена | «Редактирование»
       * | Check — та же, что в фазе загрузки. */}
      <EditingHeader
        checkDisabled={!canSave || updatePayment.isPending}
        onCancel={() => goBack(router, ROUTES.propertyPayment(propertyId, payment.id))}
        onSave={() => void save()}
      />
      {/* Горизонтальный отступ макета (705:10034, 24px) контент приносит сам —
       * PageContent его не вкладывает, как и на экране карточки платежа. */}
      <div className="flex flex-col gap-8 px-6">
        <div className="flex flex-col gap-2">
          <label htmlFor="payment-edit-amount" className="text-base font-medium leading-[18px] text-content">
            Сумма
          </label>
          <AmountBoxInput
            value={amountRaw}
            onChange={(raw) => {
              const sanitized = sanitizeAmountInput(raw);
              setAmountRaw(sanitized);
              update('amountKopecks', parseRublesToKopecks(sanitized));
            }}
            onClear={() => {
              setAmountRaw('');
              update('amountKopecks', undefined);
            }}
          />
        </div>

        <TextField
          variant="titleOut"
          title="Название платежа"
          placeholder="Введите название"
          maxLength={255}
          value={form.title}
          onChange={(event) => update('title', event.target.value)}
        />

        <FieldButton
          title="Категория"
          value={categoryLabel}
          icon={<Filter className="h-6 w-6 text-content-secondary" aria-hidden />}
          onClick={() => setCategoryOpen(true)}
        />

        <FieldButton
          title="Доход или расход"
          ariaLabel={`Доход или расход: ${TYPE_LABELS[form.type]}, нажмите, чтобы сменить`}
          value={TYPE_LABELS[form.type]}
          icon={<ChangeVertical className="h-6 w-6 text-content-secondary" aria-hidden />}
          onClick={() => update('type', togglePaymentType(form.type))}
        />

        <FieldButton
          title="Способ оплаты"
          ariaLabel={`Способ оплаты: ${FORM_OF_PAYMENT_LABELS[form.paymentForm]}, нажмите, чтобы сменить`}
          value={FORM_OF_PAYMENT_LABELS[form.paymentForm]}
          icon={<ChangeVertical className="h-6 w-6 text-content-secondary" aria-hidden />}
          onClick={() => update('paymentForm', togglePaymentForm(form.paymentForm))}
        />

        <FieldButton
          title="Регулярность платежа"
          value={recurrenceLabel(form.recurrence)}
          icon={<Calendar className="h-6 w-6 text-content-secondary" aria-hidden />}
          onClick={() => {
            setPeriodicityDraft(form.recurrence);
            setPeriodicityOpen(true);
          }}
        />

        <FieldButton
          title="Окончание платежа"
          value={form.endDate !== undefined ? formatDayMonthWithYear(form.endDate, today) : 'Бессрочно'}
          icon={<Calendar className="h-6 w-6 text-content-secondary" aria-hidden />}
          onClick={() => setEndDateOpen(true)}
        />

        {canDelete && (
          <Button
            variant="danger"
            className="h-14 w-full"
            onClick={() => setDeleteOpen(true)}
          >
            <span className="flex items-center justify-center gap-2">
              <TrashBin className="h-6 w-6" aria-hidden />
              Удалить платеж
            </span>
          </Button>
        )}
      </div>

      <StickyBottomBar>
        {/* Паддинг 24 даёт сама панель (Figma 1043:60106); дополнительная
         * обёртка давала бы двойной отступ. */}
        <Button
          className="w-full"
          disabled={!canSave}
          loading={updatePayment.isPending}
          onClick={() => void save()}
        >
          Сохранить изменения
        </Button>
      </StickyBottomBar>

      {canDelete && (
        <Modal open={deleteOpen} onOpenChange={setDeleteOpen}>
          <ModalContent
            title={payment.autoPay ? 'Удалить автоплатёж?' : 'Удалить платеж?'}
            description="Его нельзя будет восстановить. Вместо удаления платежа, его можно поставить на паузу"
          >
            {overdueCount > 0 && (
              <div className="flex items-center justify-between gap-3 py-1">
                {/* Радикс-чекбокс — кнопка (labelable): htmlFor связывает
                 * подпись с контролем, клик по тексту переключает его. */}
                <label
                  htmlFor="delete-overdue-checkbox"
                  className="cursor-pointer text-base leading-[18px] font-medium text-content"
                >
                  Удалить просроченные операции
                </label>
                <Checkbox
                  id="delete-overdue-checkbox"
                  aria-label="Удалить просроченные операции"
                  checked={deleteOverdue}
                  onCheckedChange={(checked) => setDeleteOverdue(checked === true)}
                />
              </div>
            )}
            <div className="mt-2 flex gap-3">
              <Button variant="secondary" className="flex-1" onClick={() => setDeleteOpen(false)}>
                Отменить
              </Button>
              <Button
                variant="danger"
                className="flex-1"
                loading={deletePayment.isPending}
                onClick={() => void confirmDelete()}
              >
                Удалить
              </Button>
            </div>
          </ModalContent>
        </Modal>
      )}

      {/* Рендер только в открытом состоянии — состояние ленты и черновик
          живут, пока пикер смонтирован (конвенция канона). */}
      {endDateOpen && (
        <CalendarDatePicker
          today={today}
          value={form.endDate ?? null}
          onClose={() => setEndDateOpen(false)}
          onConfirm={(date) => {
            update('endDate', date ?? undefined);
            setEndDateOpen(false);
          }}
        />
      )}
    </>
  );
}

/** Поле-кнопка (Figma «Input Field» с иконкой): бокс 56px со значением и
 * иконкой справа; тап открывает пикер в шите либо (тип, способ оплаты)
 * переключает значение на месте. У переключателей ariaLabel несёт текущее
 * значение — как у чипов шага суммы визарда. */
function FieldButton({
  title,
  value,
  icon,
  onClick,
  ariaLabel,
}: {
  readonly title: string;
  readonly value: string;
  readonly icon: JSX.Element;
  readonly onClick: () => void;
  readonly ariaLabel?: string;
}): JSX.Element {
  return (
    <div className="flex flex-col gap-2">
      <span className="text-base font-medium leading-[18px] text-content">{title}</span>
      <button
        type="button"
        aria-label={ariaLabel ?? title}
        onClick={onClick}
        className="flex h-14 w-full cursor-pointer items-center rounded-button bg-surface-muted pr-2 pl-[18px] text-left transition-shadow outline-none hover:shadow-[inset_0_0_0_2px_var(--dl-input-border)] focus-visible:ring-2 focus-visible:ring-primary"
      >
        <span className="min-w-0 flex-1 truncate text-base leading-[18px] text-content">
          {value}
        </span>
        <span className="flex h-11 w-11 shrink-0 items-center justify-center" aria-hidden>
          {icon}
        </span>
      </button>
    </div>
  );
}

/** Компактное поле суммы (Figma 1127:33146 «Сумма»): бокс 56px, группировка
 * разрядов, без символа рубля (правка владельца 2026-08-31, макет
 * 705:10034), кнопка
 * очистки — при непустом значении. */
function AmountBoxInput({
  value,
  onChange,
  onClear,
}: {
  readonly value: string;
  readonly onChange: (raw: string) => void;
  readonly onClear: () => void;
}): JSX.Element {
  const hasValue = value.length > 0;

  return (
    <div className="flex h-14 w-full items-center rounded-button bg-surface-muted pr-2 pl-[18px] transition-shadow hover:shadow-[inset_0_0_0_2px_var(--dl-input-border)]">
      <input
        id="payment-edit-amount"
        type="text"
        inputMode="decimal"
        autoComplete="off"
        spellCheck={false}
        aria-label="Сумма"
        value={groupedAmount(value)}
        placeholder="0"
        onChange={(event) => {
          const sanitized = sanitizeAmountInput(event.target.value);
          onChange(sanitized);
          syncAmountInputDom(event.target, sanitized);
        }}
        className="h-full min-w-0 flex-1 border-none bg-transparent text-base leading-[18px] text-content outline-none placeholder:text-content-secondary"
      />
      {hasValue && (
        <IconButton icon={<Cancel />} label="Очистить сумму" variant="secondary" onClick={onClear} />
      )}
    </div>
  );
}
