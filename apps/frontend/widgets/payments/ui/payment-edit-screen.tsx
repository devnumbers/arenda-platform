'use client';

import { useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { Calendar, Cancel, ChangeHorizontal, Check, Filter, Trash } from '@/shared/assets/icons';
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
} from '@/entities/payment';
import { paymentCategoryBySlug } from '@/features/payment-categories';
import { useProperty } from '@/features/properties';
import {
  buildPaymentUpdateCommand,
  editFormReady,
  FORM_OF_PAYMENT_LABELS,
  periodicityReady,
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
  Checkbox,
  groupedAmount,
  IconButton,
  Modal,
  ModalContent,
  PageContent,
  sanitizeAmountInput,
  StickyBottomBar,
  syncAmountInputDom,
  TextField,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import {
  PaymentsEmptyCard,
  PaymentsSkeleton,
  PaymentsStateCard,
} from './payments-sections';
import { CategoryStep } from './payment-create-wizard/category-step';
import { EndDateStep } from './payment-create-wizard/end-date-step';
import { PeriodicityStep } from './payment-create-wizard/periodicity-step';

/**
 * Экран правки платежа (Figma 705:10034; ранее 1127:33146, #467): форма,
 * не визард — все поля на одной странице, значения предзаполнены правилом.
 * Тип («Доход или расход») и «Способ оплаты» — строки-поля, значение
 * меняется простым нажатием по строке, без пикеров. Лейбл «Способ оплаты» —
 * по макету (решение владельца 2026-08-31), хотя глоссарий (CONTEXT-MAP)
 * считает его термином биллинга и предписывает платежам «Форму оплаты».
 * Регулярность — без «Один раз» (решение #449) с ветками дат; `since` и
 * напоминания отсутствуют (не редактируются, истории 31 спеки #453).
 * Сохранение — частичный PATCH: команда — дифф формы (update-model),
 * пересоздание планового вхождения делает сервер — правка меняет только
 * будущее. Внизу — danger-кнопка «Удалить платеж» с модалкой выбора судьбы
 * просрочек (тексты — фрейм 1127:33148): без чекбокса долг остаётся
 * (`keep_overdue=true`), с чекбоксом — сносится; после удаления — тост на
 * списке платежей объекта.
 *
 * Доступ (ADR 0028): смотрящий экрана не видит вовсе; полный доступ правит
 * без удаления; «Удалить платеж» — только владелец. Архив финансово
 * read-only (#446).
 */

type EditPicker = 'category' | 'periodicity' | 'endDate';

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

  const role = property?.access?.role;
  const canEdit =
    property !== undefined && role !== undefined && role !== 'viewer' && property.status !== 'archived';
  // Удаление — только владелец (история 49 спеки #453).
  const canDelete = role === 'owner';

  const loading = propertyQuery.isPending || paymentQuery.isPending;
  const failed = propertyQuery.isError || paymentQuery.isError;

  return (
    <>
      {!canEdit && (
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
        <div className="flex flex-col gap-8">
          {loading && (
            <>
              <PaymentsSkeleton withHeading />
              <PaymentsSkeleton withHeading />
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

          {!loading && !failed && !canEdit && (
            <div className="pt-6">
              <PaymentsEmptyCard
                title="Правка недоступна"
                hint={
                  property?.status === 'archived'
                    ? 'Объект в архиве — платежи можно только смотреть'
                    : 'У вас доступ только для просмотра этого объекта'
                }
              />
            </div>
          )}

          {!loading && !failed && canEdit && payment !== undefined && (
            <PaymentEditForm
              key={payment.id}
              propertyId={propertyId}
              payment={payment}
              canDelete={canDelete}
            />
          )}
        </div>
      </PageContent>
    </>
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

  const [picker, setPicker] = useState<EditPicker | null>(null);
  const [openBranch, setOpenBranch] = useState<PeriodicityBranch | null>(null);

  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deleteOverdue, setDeleteOverdue] = useState(false);

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

  const closePeriodicity = (): void => {
    setPicker(null);
    setOpenBranch(null);
  };

  return (
    <>
      {/* Шапка экрана правки (Figma 705:10034): Отмена | тип + «Редактирование»
       * | Check — быстрая клавиша сохранения наравне со sticky-кнопкой. */}
      <TopNav
        leading={
          <IconButton
            icon={<Cancel />}
            label="Отменить правку"
            onClick={() => goBack(router, ROUTES.propertyPayment(propertyId, payment.id))}
          />
        }
        trailing={
          <IconButton
            icon={<Check />}
            label="Сохранить"
            disabled={!canSave || updatePayment.isPending}
            onClick={() => void save()}
          />
        }
      >
        <TopNavTitle
          title={payment.autoPay ? 'Автоплатёж' : 'Платеж'}
          subtitle="Редактирование"
        />
      </TopNav>
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
          onClick={() => setPicker('category')}
        />

        <FieldButton
          title="Доход или расход"
          ariaLabel={`Доход или расход: ${TYPE_LABELS[form.type]}, нажмите, чтобы сменить`}
          value={TYPE_LABELS[form.type]}
          icon={<ChangeHorizontal className="h-6 w-6 text-content-secondary" aria-hidden />}
          onClick={() => update('type', togglePaymentType(form.type))}
        />

        <FieldButton
          title="Способ оплаты"
          ariaLabel={`Способ оплаты: ${FORM_OF_PAYMENT_LABELS[form.paymentForm]}, нажмите, чтобы сменить`}
          value={FORM_OF_PAYMENT_LABELS[form.paymentForm]}
          icon={<ChangeHorizontal className="h-6 w-6 text-content-secondary" aria-hidden />}
          onClick={() => update('paymentForm', togglePaymentForm(form.paymentForm))}
        />

        <FieldButton
          title="Регулярность платежа"
          value={recurrenceLabel(form.recurrence)}
          icon={<Calendar className="h-6 w-6 text-content-secondary" aria-hidden />}
          onClick={() => setPicker('periodicity')}
        />

        <FieldButton
          title="Окончание платежа"
          value={form.endDate !== undefined ? formatDayMonthWithYear(form.endDate, today) : 'Бессрочно'}
          icon={<Calendar className="h-6 w-6 text-content-secondary" aria-hidden />}
          onClick={() => setPicker('endDate')}
        />

        {canDelete && (
          <Button
            variant="danger"
            className="h-14 w-full"
            onClick={() => setDeleteOpen(true)}
          >
            <span className="flex items-center justify-center gap-2">
              <Trash className="h-6 w-6" aria-hidden />
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

      <Modal
        open={picker === 'category'}
        onOpenChange={(open) => !open && setPicker(null)}
      >
        <ModalContent title="Категория платежа">
          <CategoryStep
            selectedSlug={form.categorySlug ?? payment.category.slug}
            onSelect={(slug) => {
              update('categorySlug', slug);
              setPicker(null);
            }}
          />
        </ModalContent>
      </Modal>

      <Modal
        open={picker === 'periodicity'}
        onOpenChange={(open) => !open && closePeriodicity()}
      >
        <ModalContent title={periodicitySheetTitle(openBranch)}>
          <div className="-mx-6 -mb-6">
            <PeriodicityStep
              recurrence={form.recurrence}
              openBranch={openBranch}
              onOpenBranch={setOpenBranch}
              onRecurrenceChange={(recurrence) => update('recurrence', recurrence)}
              onDailyPick={() => closePeriodicity()}
              today={today}
              withHeading={false}
            />
          </div>
          {periodicityReady(form.recurrence) && (
            <div className="-mx-6 -mb-6 px-6 pb-6">
              <Button className="w-full" onClick={closePeriodicity}>
                Готово
              </Button>
            </div>
          )}
        </ModalContent>
      </Modal>

      <Modal
        open={picker === 'endDate'}
        onOpenChange={(open) => !open && setPicker(null)}
      >
        <ModalContent
          title="Окончание платежа"
          description="После выбранной даты, платеж перестанет оплачиваться и удалится. Необязательно"
        >
          <div className="-mx-6 -mb-6">
            <EndDateStep
              endDate={form.endDate}
              onEndDateChange={(endDate) => update('endDate', endDate)}
              today={today}
              withHeading={false}
            />
          </div>
        </ModalContent>
      </Modal>

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
    </>
  );
}

function periodicitySheetTitle(openBranch: PeriodicityBranch | null): string {
  switch (openBranch) {
    case 'weekdays':
    case 'monthDays':
      return 'Выберите день';
    case 'yearly':
      return 'Выберите месяц и день';
    case null:
      return 'Периодичность платежа';
  }
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
