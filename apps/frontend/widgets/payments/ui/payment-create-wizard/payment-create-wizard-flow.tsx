'use client';

import { useEffect, useRef, useState } from 'react';
import type { JSX } from 'react';
import { flushSync } from 'react-dom';
import { useRouter } from 'next/navigation';
import { ArrowLeft, Cancel, Search } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { notify } from '@/shared/lib/notifications';
import {
  Button,
  IconButton,
  PageContent,
  SearchField,
  StepsChip,
  StickyBottomBar,
  TopNav,
  TopNavTitle,
  useTabBarSuppression,
} from '@/shared/ui/design';
import type { IsoDate, Payment } from '@/entities/payment';
import { dateToIsoLocal } from '@/shared/lib/calendar';
import {
  branchKind,
  buildPaymentCreateCommand,
  draftAfterRecurrenceChange,
  periodicityReady,
  resumePaymentWizardStep,
  useCreatePayment,
  WIZARD_STEP_LAYOUTS,
  wizardDraftAfterStep,
  wizardStepReady,
  wizardTotalSteps,
  type PaymentDraftType,
  type PaymentWizardDraftController,
  type PaymentWizardMode,
  type PeriodicityBranch,
  type WizardStep,
} from '@/features/payments';
import { paymentCategoryBySlug } from '@/features/payment-categories';
import {
  emailNotificationsRowVisible,
  PAYMENTS_EMAIL_CATEGORY,
  useEmailNotificationPreferences,
} from '@/features/notifications';
import { useProperties } from '@/features/properties';
import {
  effectiveSelectedPropertyId,
  filterEditableProperties,
  propertiesByPrimaryOrder,
} from '@/entities/property';
import { AmountStep } from './amount-step';
import { CategoryStep } from './category-step';
import { PaymentSettingsStep } from './payment-settings-step';
import { PropertyStep } from './property-step';
import { BRANCH_PERIOD_LABELS, PeriodicityStep } from './periodicity-step';
import { TitleStep } from './title-step';
import {CategorySearchHint, WizardBottomBar, WizardHeading} from './wizard-chrome';
import { WizardSuccess } from './wizard-success';

/**
 * Поток шагов визарда (клиентское состояние на одном маршруте #464):
 * точка входа задаёт раскладку — у объектного входа 5 шагов (объект известен
 * из маршрута, состояние — черновик localStorage, resume на штампе шаге
 * #1055), у глобального /payments/new (решение владельца 23.10.2026) — 6,
 * после категории шаг «Выбрать объект», а состояние живёт в памяти
 * смонтированного потока (канон операций #1052: уход со страницы и
 * перезагрузка дают чистый лист). Хранилище шагов внедряется экраном —
 * draftController; монтироваться поток должен после гидрации черновика и
 * загрузки объекта родителем.
 */

export type PaymentCreateWizardFlowProps = {
  readonly mode: PaymentWizardMode;
  /** Объект входа с объекта; у глобального входа выбирается шагом
   * «Выбрать объект». */
  readonly propertyId?: string;
  readonly draftType: PaymentDraftType;
  /** Хранилище состояния шагов: черновик localStorage (объектный вход)
   * или память потока (глобальный). */
  readonly draftController: PaymentWizardDraftController;
};

export function PaymentCreateWizardFlow({
  mode,
  propertyId,
  draftType,
  draftController,
}: PaymentCreateWizardFlowProps): JSX.Element {
  const router = useRouter();
  const createPayment = useCreatePayment();
  const { draft, setDraft, clearDraft } = draftController;
  const layout = WIZARD_STEP_LAYOUTS[mode];
  // Визард — экран создания: футер глушится на всех шагах, включая те,
  // где нижняя панель «Продолжить» ещё не смонтирована (шаг 1 до выбора
  // категории, ветки шага периодичности до готовности) — ТЗ #460.
  useTabBarSuppression();
  // Успех держит и факт «название введено»: экран показывает подставленный
  // лейбл категории только когда пользователь сам набрал название
  // (правка владельца 2026-08-31).
  const [created, setCreated] = useState<{
    payment: Payment;
    hasTypedTitle: boolean;
    propertyId: string;
  } | null>(null);
  // Возобновление: у объектного входа — сохранённый штампом шаг перехода,
  // без штампа — первый незавершённый (#1055); глобальный вход всегда
  // начинает с категории.
  const [step, setStep] = useState<WizardStep>(() =>
    mode === 'global' ? layout.category : resumePaymentWizardStep(draft),
  );
  // Автоподъём клавиатуры на шаге суммы (#1151, research #1148 §A): флаг
  // ставит только переход-жест («Далее» шага настроек → шаг суммы); маунты
  // вне жеста — возобновление черновика сразу на шаге суммы, прямой роут —
  // без флага: iOS клавиатуру вне жеста не поднимает (WebKit bug 195884),
  // а предфокус без клавиатуры делал бы мёртвым тап по дисплею вне цифр.
  const [amountFocusOnMount, setAmountFocusOnMount] = useState(false);
  const [openBranch, setOpenBranch] = useState<PeriodicityBranch | null>(null);
  // Поиск категорий живёт в хедере шага 1 (Figma 781:12299): лупа меняет
  // чип «Шаг N из M» на поле, «Назад» возвращает чип и сбрасывает запрос.
  const [categorySearchOpen, setCategorySearchOpen] = useState(false);
  const [categoryQuery, setCategoryQuery] = useState('');
  const searchInputRef = useRef<HTMLInputElement | null>(null);
  // Мастер видимости ряда почты (карта #1551): решение для команды сабмита.
  // Общий предикат с NotifyEmailRow шага 4 — расхождений нет; в полёте
  // решение не блокируем (галочка едет), при off и сбое ряд скрыт и поле
  // в команду не едёт (канон #1552). Кэш общий — второй запрос не летает.
  const emailMasterQuery = useEmailNotificationPreferences();
  const notifyEmailRowVisible = emailNotificationsRowVisible(
    emailMasterQuery.isPending,
    emailMasterQuery.data?.[PAYMENTS_EMAIL_CATEGORY],
  );
  // Справочник для шага «Выбрать объект» и дефолта предвыбора — только у
  // глобального входа; у объектного маршрут сам несёт объект.
  const propertiesQuery = useProperties({ enabled: mode === 'global' });
  const editableProperties = propertiesByPrimaryOrder(
    filterEditableProperties(propertiesQuery.data ?? []),
  );
  // Видимый на шаге и уходящий в сабмит объект: явный выбор пользователя
  // сильнее, до явного — дефолт (верхняя строка, канон «Основного
  // объекта»); в черновике хранится только явный выбор.
  const selectedPropertyId = effectiveSelectedPropertyId(
    draft.propertyId,
    editableProperties,
  );


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

  const today: IsoDate = dateToIsoLocal(new Date());

  if (created !== null) {
    return (
      <>
        <TopNav
          leading={
            <IconButton icon={<Cancel />} label="Закрыть" onClick={closeAfterCreation} />
          }
        />
        <PageContent>
          <div className="pt-16">
            <WizardSuccess
              propertyId={created.propertyId}
              created={created.payment}
              hasTypedTitle={created.hasTypedTitle}
              draftType={draftType}
              onClose={closeAfterCreation}
            />
          </div>
        </PageContent>
      </>
    );
  }

  return (
    <>
      <TopNav
        variant={step === layout.category && categorySearchOpen ? 'search' : 'default'}
        leading={<IconButton icon={<ArrowLeft />} label="Назад" onClick={navigateBack} />}
        trailing={
          step === layout.category && !categorySearchOpen ? (
            <IconButton
              icon={<Search />}
              label="Поиск по категориям"
              onClick={() => setCategorySearchOpen(true)}
            />
          ) : undefined
        }
      >
        {step === layout.category && categorySearchOpen ? (
          <SearchField
            focusOnMount
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
        ) : step === layout.periodicity && openBranch !== null ? (
          /* Ветка шага периодичности сменяет чип на подпись типа периода
             (Figma 1056:52895). */
          <TopNavTitle title={BRANCH_PERIOD_LABELS[openBranch]} />
        ) : (
          <StepsChip step={step} total={wizardTotalSteps(mode)} size="m" />
        )}
      </TopNav>
      {/* У макета контентный фрейм несёт pt-72 под плавающий TopNav (44px);
          здесь хедер в потоке, так что от навигации до заголовка остаётся
          28+24 = 52px — как во Figma. */}
      <PageContent className="pt-0">
        {step === layout.category && (
          <>
            {/* Открытый поиск меняет контент (Figma 1049:46256/46418):
                заголовок — только при закрытом поиске; с открытым — пустой
                запрос даёт иллюстрацию-подсказку, запрос — отфильтрованный
                список без заголовка (решение владельца 06.10, #1152). */}
            {categorySearchOpen ? (
              categoryQuery === '' ? (
                <CategorySearchHint text="Начните искать категорию" />
              ) : (
                <CategoryStep
                  selectedSlug={draft.categorySlug}
                  onSelect={(slug) => setDraft((prev) => ({ ...prev, categorySlug: slug }))}
                  query={categoryQuery}
                />
              )
            ) : (
              <>
                <WizardHeading title="Выберите категорию платежа" variant="h1" />
                <CategoryStep
                  selectedSlug={draft.categorySlug}
                  onSelect={(slug) => setDraft((prev) => ({ ...prev, categorySlug: slug }))}
                />
              </>
            )}
            {/* «Продолжить» — после выбора категории (Figma 823:4243). */}
            {draft.categorySlug !== undefined && (
              <StickyBottomBar>
                <WizardBottomBar>
                  <Button
                    className="w-full"
                    onClick={() => {
                      setCategorySearchOpen(false);
                      setCategoryQuery('');
                      // После категории глобальный вход ведёт на «Выбрать
                      // объект», объектный — сразу на название.
                      goToStep(
                        mode === 'global'
                          ? WIZARD_STEP_LAYOUTS.global.property
                          : layout.title,
                      );
                    }}
                  >
                    Продолжить
                  </Button>
                </WizardBottomBar>
              </StickyBottomBar>
            )}
          </>
        )}
        {/* Шаг «Выбрать объект» существует только у глобального входа
            (аннотация фрейма выбора #570): у объектного маршрута объект
            задан самим входом. Дефолт предвыбора — верхняя строка, явный
            выбор сильнее. */}
        {mode === 'global' && step === WIZARD_STEP_LAYOUTS.global.property && (
          <>
            <WizardHeading title="Выберите объект" variant="h1" />
            <PropertyStep
              selectedPropertyId={selectedPropertyId}
              onSelect={(id) => setDraft((prev) => ({ ...prev, propertyId: id }))}
              ariaLabel="Объект платежа"
            />
            {/* «Продолжить» — когда объект известен: с предвыбором панель
                видна сразу при входе на шаг, исчезает только на пустом
                справочнике редактируемых (канон §3: панель не показывается,
                пока действия нет). */}
            {wizardStepReady('property', { ...draft, propertyId: selectedPropertyId }) && (
              <StickyBottomBar>
                <WizardBottomBar>
                  <Button className="w-full" onClick={() => goToStep(WIZARD_STEP_LAYOUTS.global.title)}>
                    Продолжить
                  </Button>
                </WizardBottomBar>
              </StickyBottomBar>
            )}
          </>
        )}
        {step === layout.title && (
          <>
            <TitleStep
              title={draft.title ?? ''}
              onTitleChange={(title) => setDraft((prev) => ({ ...prev, title }))}
            />
            <StickyBottomBar>
              <WizardBottomBar>
                <Button className="w-full" onClick={() => goToStep(layout.periodicity)}>
                  Продолжить
                </Button>
              </WizardBottomBar>
            </StickyBottomBar>
          </>
        )}
        {step === layout.periodicity && (
          <>
            <PeriodicityStep
              recurrence={draft.recurrence}
              openBranch={openBranch}
              onOpenBranch={setOpenBranch}
              onRecurrenceChange={(recurrence) =>
                // Смена периодичности чистит стоящее окончание, ставшее
                // раньше первого вхождения нового расписания (#1155,
                // решение владельца 2026-10-06 — как в правке).
                setDraft((prev) => draftAfterRecurrenceChange(prev, recurrence, today))
              }
              onDailyPick={() => goToStep(layout.settings)}
              onYearlyConfirm={() => goToStep(layout.settings)}
              today={today}
            />
            {/* Панель шага видна, когда периодичность готова и совпадает с
                открытой веткой: в ветке недели — после первого выбранного
                дня (Figma 1056:52895), месяц готовит дефолт сразу; год
                подтверждается в календаре («Продолжить» сразу ведёт на
                настройки), панель на меню — точка возврата к готовому
                правилу. */}
            {periodicityReady(draft.recurrence)
              && (openBranch === null || openBranch === branchKind(draft.recurrence)) && (
              <StickyBottomBar>
                <WizardBottomBar>
                  <Button className="w-full" onClick={() => goToStep(layout.settings)}>
                    Продолжить
                  </Button>
                </WizardBottomBar>
              </StickyBottomBar>
            )}
          </>
        )}
        {step === layout.settings && (
          <>
            <PaymentSettingsStep
              draftType={draftType}
              reminderOffsetDays={draft.reminderOffsetDays}
              onReminderOffsetDaysChange={(reminderOffsetDays) =>
                setDraft((prev) => ({ ...prev, reminderOffsetDays }))
              }
              notifyAutoPaid={draft.notifyAutoPaid}
              onNotifyAutoPaidChange={(notifyAutoPaid) =>
                setDraft((prev) => ({ ...prev, notifyAutoPaid }))
              }
              notifyEmail={draft.notifyEmail ?? true}
              onNotifyEmailChange={(notifyEmail) =>
                setDraft((prev) => ({ ...prev, notifyEmail }))
              }
              endDate={draft.endDate}
              onEndDateChange={(endDate) => setDraft((prev) => ({ ...prev, endDate }))}
              recurrence={draft.recurrence}
              today={today}
            />
            <StickyBottomBar>
              <WizardBottomBar>
                <Button className="w-full" onClick={() => goToStep(layout.amount)}>
                  Далее
                </Button>
              </WizardBottomBar>
            </StickyBottomBar>
          </>
        )}
        {step === layout.amount && (
          <>
            <AmountStep
              focusOnMount={amountFocusOnMount}
              amountKopecks={draft.amountKopecks}
              onAmountChange={(amountKopecks) =>
                setDraft((prev) => ({ ...prev, amountKopecks }))
              }
              type={draft.type}
              onTypeChange={(type) => setDraft((prev) => ({ ...prev, type }))}
            />
            <StickyBottomBar>
              <WizardBottomBar>
                <Button
                  className="w-full"
                  disabled={!wizardStepReady('amount', draft)}
                  loading={createPayment.isPending}
                  onClick={() => void submit()}
                >
                  {draftType === 'autopayment' ? 'Создать автоплатеж' : 'Создать платеж'}
                </Button>
              </WizardBottomBar>
            </StickyBottomBar>
          </>
        )}
      </PageContent>
    </>
  );

  function closeAfterCreation(): void {
    goBack(router, fallbackHref());
  }

  function navigateBack(): void {
    if (step === layout.periodicity && openBranch !== null) {
      setOpenBranch(null);
      return;
    }
    if (step === layout.category && categorySearchOpen) {
      setCategorySearchOpen(false);
      setCategoryQuery('');
      return;
    }
    if (step > layout.category) {
      const prev = (step - 1) as WizardStep;
      setStep(prev);
      // Уход с шага суммы гасит маунт-фокус: следующий вход на шаг —
      // только новым жестом (#1151).
      setAmountFocusOnMount(false);
      // «Назад» — тоже переход: штамп шага едет в черновик (#1055); у
      // глобального входа черновик в памяти, штамп просто стирается вместе
      // с потоком.
      setDraft((prevDraft) => wizardDraftAfterStep(prevDraft, prev));
      return;
    }
    goBack(router, fallbackHref());
  }

  function goToStep(next: WizardStep): void {
    // Общая семантика перехода: шаг + штамп в черновике (#1055); ветка
    // периодичности гасится вне своего шага.
    const commit = (): void => {
      setStep(next);
      // Штамп шага в пейлоаде: перезагрузка возвращает на этот же шаг
      // (#1055, объектный вход).
      setDraft((prev) => wizardDraftAfterStep(prev, next));
      if (next !== layout.periodicity) {
        setOpenBranch(null);
      }
      if (next === layout.amount) {
        setAmountFocusOnMount(true);
      }
    };
    if (next === layout.amount) {
      // Синхронный коммит внутри жеста «Далее» (research #1148 §A): шаг
      // суммы монтируется и его маунт-фокус (callback-ref инпута) срабатывает
      // в той же задаче event loop, что и клик, — только тогда iOS поднимает
      // клавиатуру. flushSync в обработчике события — документированно-
      // безопасное место (react.dev/flushSync).
      flushSync(commit);
      return;
    }
    commit();
  }

  function fallbackHref(): string {
    return mode === 'property' && propertyId !== undefined
      ? ROUTES.propertyPayments(propertyId)
      : ROUTES.payments;
  }

  /** Объект сабмита: у входа с объекта — маршрут, у глобального — видимый
   * выбор шага «Выбрать объект» (явный или дефолт). */
  function targetPropertyId(): string | undefined {
    return mode === 'property' ? propertyId : selectedPropertyId;
  }

  async function submit(): Promise<void> {
    const resolvedPropertyId = targetPropertyId();
    if (resolvedPropertyId === undefined) {
      return;
    }
    // Факт «название введено» — из черновика на момент сабмита: в ответе API
    // название уже всегда заполнено (пустое поле замещает лейбл категории).
    const hasTypedTitle = draft.title !== undefined && draft.title.trim().length > 0;
    const command = buildPaymentCreateCommand(draft, {
      autoPay: draftType === 'autopayment',
      resolveTitle: (slug) => paymentCategoryBySlug(slug)?.label,
      notifyEmailRowVisible,
    });
    if (command === undefined) {
      return;
    }
    try {
      const payment = await createPayment.mutateAsync({
        propertyId: resolvedPropertyId,
        command,
      });
      clearDraft();
      notify.scenarios.payments.created();
      setCreated({ payment, hasTypedTitle, propertyId: resolvedPropertyId });
    } catch (error) {
      notify.scenarios.payments.createError(error);
    }
  }
}
