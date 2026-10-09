'use client';

import { useEffect, useRef, useState, type ComponentProps, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import {
  ArrowLeft,
  Calendar,
  Cancel,
  Check,
  Search,
  SmallArrowDown,
  TrashBin,
} from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import {
  kopecksToRublesString,
  parseRublesToKopecks,
} from '@/shared/lib/format-money';
import { goBack } from '@/shared/lib/navigation';
import { notify } from '@/shared/lib/notifications';
import { dateToIsoLocal } from '@/shared/lib/calendar';
import {
  formatDayMonthWithYear,
  firstOccurrence,
  paymentReminderOptionLabel,
  PAYMENT_REMINDER_OPTIONS,
  recurrenceLabel,
  type IsoDate,
  type Payment,
  type Recurrence,
} from '@/entities/payment';
import { paymentCategoryBySlug } from '@/features/payment-categories';
import { propertyPermissions } from '@/entities/property';
import { useMe } from '@/features/auth';
import { useProperty } from '@/features/properties';
import { useRentals } from '@/features/rentals';
import {
  emailReminderCaption,
  EmailNotificationsRow,
} from '@/features/notifications';
import {
  buildPaymentUpdateCommand,
  branchKind,
  editFormReady,
  formAfterRecurrenceChange,
  periodicityReady,
  recurrencesEqual,
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
  PickerMenu,
  sanitizeAmountInput,
  SearchField,
  StickyBottomBar,
  syncAmountInputDom,
  TextField,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import {
  PaymentsStateCard,
} from './payments-sections';
import { PaymentEditFormSkeleton } from './payments-skeletons';
import { CategoryStep } from './payment-create-wizard/category-step';
import {
  BRANCH_PERIOD_LABELS,
  PeriodicityStep,
} from './payment-create-wizard/periodicity-step';
import { CategorySearchHint, WizardDirectionSegment, WizardHeading } from './payment-create-wizard/wizard-chrome';

/**
 * Экран правки платежа по макетам 1127-32083 (форма) и 1130-35022 (шит
 * «Напоминать о платеже», #1197; ранее 705:10034, #467): форма, не визард —
 * все поля на одной странице, значения предзаполнены правилом. Хедер —
 * «Платеж» с подписью «Редактирование» (макет 1127:32086). Направление —
 * сегмент «Расход/Доход» под суммой (макет 1127:32742, во всю колонку);
 * «Способ оплаты» из макета не рисуется — поле снесено из контракта и
 * продукта (решение владельца 07.10: #1009 остаётся закрытым, ADR 0047
 * рев. №3). Подписи полей — формулировки макета 1130 («Название платежа»,
 * «Регулярность платежа», «Окончание платежа», «Напоминать о платеже»),
 * согласованные с шагами создания. Категория — отдельная страница на том
 * же маршруте, как шаг 1 визарда (заголовок — H1 «Выберите категорию
 * платежа», #1152); уход со страницы не теряет несохранённые правки формы.
 * Периодичность — отдельная страница как шаг 3 визарда без контентных
 * заголовков (решение владельца 07.10, #1192); поведение выровнено
 * с созданием (#1153): «Каждый день» и «Каждый год» («Продолжить»
 * календаря) применяются сразу и закрывают страницу, ветки недели и
 * месяца живут в черновике страницы и применяются кнопкой «Выбрать» —
 * «Назад» черновик отбрасывает. Окончание — канонический бесконечный
 * календарь поверх формы (решение владельца 2026-09-04), заголовок
 * пикера — название поля: дни раньше первого вхождения действующего
 * расписания недоступны (инвариант «окна графика» #1150), а смена
 * периодичности, сделавшая стоящее окончание невалидным, сбрасывает его
 * молча (решение владельца 2026-10-06, #1155) — сохранение уходит с
 * tri-state endDate: null.
 *
 * Напоминание (#1197) — редактируемое: строка «Напоминать о платеже»
 * открывает канонный PickerMenu (шит на мобиле, меню на ПК) с опциями
 * «За 1 / 3 / 7 дней» по макету 1130-35022 и четвёртой «Не напоминать» —
 * краевой случай правила без напоминания (NULL в контракте): она же
 * снимает напоминание (tri-state reminderOffsetDays: null). У
 * автоплатёжного правила вместо пикера оффсета — строка «Уведомлять
 * об оплате» с радио «Не уведомлять»/«Да, уведомлять» — пер-платёжный
 * флаг notifyAutoPaid (#1189), подписи шага 4 создания (#1193); радио
 * напоминания у автоплатежа нет (как на шаге 4, решение гриллинга
 * #1186). Регулярность — без «Один раз» (решение #449) с ветками дат;
 * `since` не редактируется. Ряд «Уведомления на почту» — общий
 * EmailNotificationsRow (шоткат глобальной email-настройки категории,
 * канон #746, подпись — как на шаге 4 создания).
 *
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
 * карточке «Правка недоступна» с арендной подсказкой — и карточка не тупик:
 * кнопка «Условия аренды» ведёт прямо в условия аренды (#988), у
 * завершённой — в архивную карточку с подзаголовком «В архиве» (#535).
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

  // Переход вместо тупика (#988): карточка недоступности ведёт в условия
  // аренды — найдём её по связи rentPayment.paymentId (ADR 0053 §4);
  // завершённая живёт в архиве терминов с ?rental= (#535).
  const rentalsQuery = useRentals(propertyId, { enabled: managed });
  const managedRental = managed
    ? rentalsQuery.data?.find((rental) => rental.rentPayment.paymentId === paymentId)
    : undefined;

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
              <PaymentsStateCard
                title="Правка недоступна"
                hint={
                  managed
                    ? 'Платёж управляется арендой — изменить его можно только в аренде'
                    : property?.status === 'archived'
                      ? 'Объект в архиве — платежи можно только смотреть'
                      : 'У вас доступ только для просмотра этого объекта'
                }
                // У управляемого платежа карточка не тупик: условия правятся
                // на аренде (#988) — кнопка ведёт туда, пока аренда найдена.
                // White на серой карточке (secondary был бы muted-on-muted,
                // приёмка #988) — прецедент «Продлить» в PropertyRentalBlock.
                action={
                  managed && managedRental !== undefined ? (
                    <Button
                      variant="white"
                      size="small"
                      radius="m"
                      onClick={() =>
                        router.push(
                          managedRental.status === 'completed'
                            ? ROUTES.propertyRentalCompletedTerms(propertyId, managedRental.id)
                            : ROUTES.propertyRentalTerms(propertyId),
                        )
                      }
                    >
                      Условия аренды
                    </Button>
                  ) : undefined
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
            Сохранить
          </Button>
        </StickyBottomBar>
      )}
    </>
  );
}

/** Шапка экрана правки (макет 1127:32086): Отмена | «Платеж» с подписью
 * «Редактирование» | Check — быстрая клавиша сохранения наравне
 * со sticky-кнопкой. Общая форме и фазе загрузки — хедер не меняется,
 * когда данные пришли. */
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
      <TopNavTitle title="Платеж" subtitle="Редактирование" />
    </TopNav>
  );
}

/** Заголовок формы: значение предзаполнено правилом, пустой ввод заменяет
 * лейбл категории при сохранении (правила визарда #464). Напоминание
 * и флаг автоплатежа едут в форму целиком — дифф считает update-model. */
function formFromPayment(payment: Payment): PaymentEditForm {
  return {
    type: payment.type,
    title: payment.title,
    amountKopecks: payment.amountKopecks,
    categorySlug: undefined,
    recurrence: payment.recurrence,
    endDate: payment.endDate,
    reminderOffsetDays: payment.reminderOffsetDays,
    notifyAutoPaid: payment.notifyAutoPaid,
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
  // Ряд «Уведомления на почту» (макет 1127:32504) — общий EmailNotificationsRow:
  // подпись с почтой пользователя, как на шаге 4 создания (#1193).
  const meQuery = useMe();
  const email = meQuery.data?.email ?? null;

  const today: IsoDate = dateToIsoLocal(new Date());
  const resolveTitle = (slug: string): string | undefined =>
    paymentCategoryBySlug(slug)?.label;

  const [form, setForm] = useState<PaymentEditForm>(() => formFromPayment(payment));
  const [amountRaw, setAmountRaw] = useState<string>(() => initialAmountRaw(payment.amountKopecks));

  const [endDateOpen, setEndDateOpen] = useState(false);
  const [openBranch, setOpenBranch] = useState<PeriodicityBranch | null>(null);
  // Периодичность — отдельная страница на том же маршруте (как шаг 3
  // визарда). Ветки недели и месяца живут в черновике страницы и попадают
  // в форму только по кнопке «Выбрать» — иначе незавершённый период
  // («Каждую неделю в —») оставался бы в форме при выходе назад. «Каждый
  // день» и «Каждый год» применяются сразу, как в создании (#1153).
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
    // Черновик отбрасывается: ветки недели/месяца меняются только кнопкой
    // «Выбрать».
    setPeriodicityOpen(false);
    setOpenBranch(null);
    setPeriodicityDraft(undefined);
  };

  // Мгновенное применение (выровнено с созданием, #1153): «Каждый день»
  // готов сразу, годовое правило приходит подтверждением календаря
  // («Продолжить») — значение передаётся аргументом, черновик страницы к
  // моменту колбэка ещё не обновлён. Применение чистит стоящее окончание,
  // ставшее раньше первого вхождения нового расписания (#1155, решение
  // владельца 2026-10-06: сброс молча, без подсказки; сохранение уйдёт
  // с tri-state endDate: null).
  const applyRecurrenceNow = (recurrence: Recurrence): void => {
    setForm((prev) => formAfterRecurrenceChange(prev, recurrence, payment));
    closePeriodicityPage();
  };

  const applyPeriodicity = (): void => {
    if (periodicityDraft === undefined) {
      return;
    }
    applyRecurrenceNow(periodicityDraft);
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
  // Прошлое закрыто, задним числом дата не назначается. Минимум пикера —
  // первое вхождение действующего расписания (endDate в расчёт не берётся):
  // инвариант «окна графика» #1150, заголовок — название поля, канон
  // аренды (#1155).

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
          variant={categorySearchOpen ? 'search' : 'default'}
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
          ) : undefined}
        </TopNav>
        {/* КатегорияStep и подсказка поиска приносят свои отступы (шаг визарда
         * рассчитан на полноширинный контент) — обёртке паддинг не нужен.
         * Заголовок — только при закрытом поиске (макет 1049:46418,
         * решение владельца 06.10): открытый поиск показывает подсказку
         * или отфильтрованный список без заголовка. Заголовок и список —
         * один блок: родительский gap-8 не должен разносить их. */}
        {categorySearchOpen ? (
          categoryQuery === '' ? (
            <CategorySearchHint text="Начните искать категорию" />
          ) : (
            <CategoryStep
              selectedSlug={selectedSlug}
              onSelect={(slug) => update('categorySlug', slug)}
              query={categoryQuery}
            />
          )
        ) : (
          <div>
            <WizardHeading title="Выберите категорию платежа" variant="h1" />
            <CategoryStep
              selectedSlug={selectedSlug}
              onSelect={(slug) => update('categorySlug', slug)}
            />
          </div>
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

  // Страница периодичности — как шаг 3 визарда (Figma 3213:71290), но без
  // контентных заголовков: их нет и в макете правки, хедер ведёт названием
  // периода/«Выбор периодичности» (решение владельца 07.10, #1192). День и
  // год применяются сразу (#1153, как в создании #995); ветки
  // недели/месяца — в черновике, «Выбрать» применяется когда периодичность
  // готова, ветка закрыта или совпадает с ней и значение изменилось.
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
          onDailyPick={() => applyRecurrenceNow({ kind: 'daily' })}
          onYearlyConfirm={applyRecurrenceNow}
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
      {/* Шапка EditingHeader (макет 1127:32086): Отмена | «Платеж /
       * Редактирование» | Check — та же, что в фазе загрузки. */}
      <EditingHeader
        checkDisabled={!canSave || updatePayment.isPending}
        onCancel={() => goBack(router, ROUTES.propertyPayment(propertyId, payment.id))}
        onSave={() => void save()}
      />
      {/* Горизонтальный отступ макета (1127:32088, 24px) контент приносит
       * сам; ряд email-уведомлений — full-bleed (px-6 внутри канонного
       * ряда), удаление — в собственной обёртке. Группа «Сумма + сегмент»
       * держит внутренний зазор 8px, между группами полей — 32px (gap-8). */}
      <div className="flex flex-col gap-8">
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
            {/* Направление — сегмент «Расход/Доход» под суммой (макет
             * 1127:32742, во всю колонку) вместо строки-переключателя. */}
            <WizardDirectionSegment
              type={form.type}
              onTypeChange={(type) => update('type', type)}
              ariaLabel="Направление платежа"
              fullWidth
            />
          </div>

          <TextField
            variant="titleOut"
            title="Название платежа"
            placeholder="Введите название"
            description="Необязательно"
            maxLength={255}
            value={form.title}
            onChange={(event) => update('title', event.target.value)}
          />

          {/* Категория — отдельная страница на том же маршруте (как шаг 1
           * визарда); шеврон по макету 1127:32326. */}
          <FieldButton
            title="Категория"
            value={categoryLabel}
            icon={<SmallArrowDown className="h-6 w-6 text-content-secondary" aria-hidden />}
            onClick={() => setCategoryOpen(true)}
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
            hint="Необязательно"
            onClick={() => setEndDateOpen(true)}
          />

          {/* Напоминание (#1197): у ручного правила — пикер оффсета (шит
           * 1130-35022: «За 1 / 3 / 7 дней» + четвёртой «Не напоминать»
           * для правила без напоминания), у автоплатёжа — «Уведомлять
           * об оплате» да/нет (#1189), подписи шага 4 создания. */}
          {payment.autoPay ? (
            <PickerMenu
              title="Уведомлять об оплате"
              groups={[
                {
                  options: [
                    {
                      label: 'Не уведомлять',
                      selected: form.notifyAutoPaid !== true,
                      onSelect: () => update('notifyAutoPaid', false),
                    },
                    {
                      label: 'Да, уведомлять',
                      selected: form.notifyAutoPaid === true,
                      onSelect: () => update('notifyAutoPaid', true),
                    },
                  ],
                },
              ]}
            >
              <FieldButton
                title="Уведомлять об оплате"
                value={form.notifyAutoPaid ? 'Да, уведомлять' : 'Не уведомлять'}
                icon={<SmallArrowDown className="h-6 w-6 text-content-secondary" aria-hidden />}
                onClick={() => {}}
              />
            </PickerMenu>
          ) : (
            <PickerMenu
              title="Напоминать о платеже"
              groups={[
                {
                  options: [
                    ...PAYMENT_REMINDER_OPTIONS.map((option) => ({
                      label: option.label,
                      selected: form.reminderOffsetDays === option.offset,
                      onSelect: () => update('reminderOffsetDays', option.offset),
                    })),
                    {
                      label: 'Не напоминать',
                      selected: form.reminderOffsetDays === undefined,
                      onSelect: () => update('reminderOffsetDays', undefined),
                    },
                  ],
                },
              ]}
            >
              <FieldButton
                title="Напоминать о платеже"
                value={
                  form.reminderOffsetDays === undefined
                    ? 'Не напоминать'
                    : paymentReminderOptionLabel(form.reminderOffsetDays)
                }
                icon={<SmallArrowDown className="h-6 w-6 text-content-secondary" aria-hidden />}
                onClick={() => {}}
              />
            </PickerMenu>
          )}
        </div>

        <EmailNotificationsRow
          title="Уведомления на почту"
          caption={emailReminderCaption('о платеже', email)}
        />

        {canDelete && (
          <div className="px-6">
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
          </div>
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
          Сохранить
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
          title="Окончание платежа"
          today={today}
          value={form.endDate ?? null}
          minDate={
            firstOccurrence({
              recurrence: form.recurrence,
              since: payment.since,
              endDate: undefined,
              pauses: payment.pauses,
            }) ?? undefined
          }
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

/** Поле-кнопка (макет «Input Field» с иконкой): бокс 56px со значением и
 * иконкой справа; опциональная строка-подсказка под боксом (13/15, макет
 * 1127:32404 «Необязательно»). Тап открывает пикер в шите либо (тип)
 * переключает значение на месте. Работает и триггером PickerMenu (asChild):
 * остальные пропсы кнопки — aria/ref от Radix — прокидываются на
 * <button> (канон PickerTriggerBox). */
function FieldButton({
  title,
  value,
  icon,
  onClick,
  ariaLabel,
  hint,
  ...rest
}: {
  readonly title: string;
  readonly value: string;
  readonly icon: JSX.Element;
  readonly onClick: () => void;
  readonly ariaLabel?: string;
  readonly hint?: string;
} & Omit<ComponentProps<'button'>, 'title' | 'type' | 'onClick' | 'aria-label' | 'children'>): JSX.Element {
  return (
    <div className="flex flex-col gap-2">
      <span className="text-base font-medium leading-[18px] text-content">{title}</span>
      <button
        type="button"
        aria-label={ariaLabel ?? title}
        onClick={onClick}
        {...rest}
        className="flex h-14 w-full cursor-pointer items-center rounded-button bg-surface-muted pr-2 pl-[18px] text-left transition-shadow outline-none hover:shadow-[inset_0_0_0_2px_var(--dl-input-border)] focus-visible:ring-2 focus-visible:ring-primary"
      >
        <span className="min-w-0 flex-1 truncate text-base leading-[18px] text-content">
          {value}
        </span>
        <span className="flex h-11 w-11 shrink-0 items-center justify-center" aria-hidden>
          {icon}
        </span>
      </button>
      {hint !== undefined && (
        <span className="text-[13px] leading-[15px] text-content-tertiary">{hint}</span>
      )}
    </div>
  );
}

/** Компактное поле суммы (макет 1127:32741 «Сумма»): бокс 56px, группировка
 * разрядов, суффикс «₽» (макет 1127-32083 вернул символ — ранее снят
 * решением 2026-08-31 по макету 705:10034), кнопка очистки — при непустом
 * значении. */
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
      <span className="pr-1 text-base leading-[18px] text-content" aria-hidden>
        ₽
      </span>
      {hasValue && (
        <IconButton icon={<Cancel />} label="Очистить сумму" variant="secondary" onClick={onClear} />
      )}
    </div>
  );
}
