'use client';

import { useEffect, useRef, useState } from 'react';
import type { JSX } from 'react';
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
  periodicityReady,
  resumePaymentWizardStep,
  useCreatePayment,
  usePaymentWizardDraft,
  WIZARD_TOTAL_STEPS,
  wizardDraftAfterStep,
  wizardStepReady,
  type PaymentDraftType,
  type PeriodicityBranch,
  type WizardStep,
} from '@/features/payments';
import { paymentCategoryBySlug } from '@/features/payment-categories';
import { AmountStep } from './amount-step';
import { CategoryStep } from './category-step';
import { PaymentSettingsStep } from './payment-settings-step';
import { BRANCH_PERIOD_LABELS, PeriodicityStep } from './periodicity-step';
import { TitleStep } from './title-step';
import {CategorySearchHint, WizardBottomBar, WizardHeading} from './wizard-chrome';
import { WizardSuccess } from './wizard-success';

/**
 * Поток шагов визарда (клиентское состояние на одном маршруте #464):
 * монтируется после гидрации черновика и загрузки объекта родителем,
 * возобновляется на сохранённом штампом шаге (#1055), а без штампа —
 * на первом незавершённом; черновик живёт в localStorage per объект+тип
 * (история 10 спеки #453).
 */

export type PaymentCreateWizardFlowProps = {
  readonly propertyId: string;
  readonly draftType: PaymentDraftType;
};

export function PaymentCreateWizardFlow({
  propertyId,
  draftType,
}: PaymentCreateWizardFlowProps): JSX.Element {
  const router = useRouter();
  const createPayment = useCreatePayment(propertyId);
  const { draft, setDraft, clearDraft } = usePaymentWizardDraft(propertyId, draftType);
  // Визард — экран создания: футер глушится на всех шагах, включая те,
  // где нижняя панель «Продолжить» ещё не смонтирована (шаг 1 до выбора
  // категории, ветки шага 3 до готовности) — ТЗ #460.
  useTabBarSuppression();
  // Успех держит и факт «название введено»: экран показывает подставленный
  // лейбл категории только когда пользователь сам набрал название
  // (правка владельца 2026-08-31).
  const [created, setCreated] = useState<{
    payment: Payment;
    hasTypedTitle: boolean;
  } | null>(null);
  // Возобновление: сохранённый штампом шаг перехода, без штампа — первый
  // незавершённый (#1055).
  const [step, setStep] = useState<WizardStep>(() => resumePaymentWizardStep(draft));
  const [openBranch, setOpenBranch] = useState<PeriodicityBranch | null>(null);
  // Поиск категорий живёт в хедере шага 1 (Figma 781:12299): лупа меняет
  // чип «Шаг N из 5» на поле, «Назад» возвращает чип и сбрасывает запрос.
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
              propertyId={propertyId}
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
        variant={step === 1 && categorySearchOpen ? 'search' : 'default'}
        leading={<IconButton icon={<ArrowLeft />} label="Назад" onClick={navigateBack} />}
        trailing={
          step === 1 && !categorySearchOpen ? (
            <IconButton
              icon={<Search />}
              label="Поиск по категориям"
              onClick={() => setCategorySearchOpen(true)}
            />
          ) : undefined
        }
      >
        {step === 1 && categorySearchOpen ? (
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
        ) : step === 3 && openBranch !== null ? (
          /* Ветка шага 3 сменяет чип на подпись типа периода
             (Figma 1056:52895). */
          <TopNavTitle title={BRANCH_PERIOD_LABELS[openBranch]} />
        ) : (
          <StepsChip step={step} total={WIZARD_TOTAL_STEPS} size="m" />
        )}
      </TopNav>
      {/* У макета контентный фрейм несёт pt-72 под плавающий TopNav (44px);
          здесь хедер в потоке, так что от навигации до заголовка остаётся
          28+24 = 52px — как во Figma. */}
      <PageContent className="pt-0">
        {step === 1 && (
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
                      goToStep(2);
                    }}
                  >
                    Продолжить
                  </Button>
                </WizardBottomBar>
              </StickyBottomBar>
            )}
          </>
        )}
        {step === 2 && (
          <>
            <TitleStep
              title={draft.title ?? ''}
              onTitleChange={(title) => setDraft((prev) => ({ ...prev, title }))}
            />
            <StickyBottomBar>
              <WizardBottomBar>
                <Button className="w-full" onClick={() => goToStep(3)}>
                  Продолжить
                </Button>
              </WizardBottomBar>
            </StickyBottomBar>
          </>
        )}
        {step === 3 && (
          <>
            <PeriodicityStep
              recurrence={draft.recurrence}
              openBranch={openBranch}
              onOpenBranch={setOpenBranch}
              onRecurrenceChange={(recurrence) =>
                setDraft((prev) => ({ ...prev, recurrence }))
              }
              onDailyPick={() => goToStep(4)}
              onYearlyConfirm={() => goToStep(4)}
              today={today}
            />
            {/* Панель шага видна, когда периодичность готова и совпадает с
                открытой веткой: в ветке недели — после первого выбранного
                дня (Figma 1056:52895), месяц готовит дефолт сразу; год
                подтверждается в календаре («Продолжить» сразу ведёт на
                шаг 4), панель на меню — точка возврата к готовому
                правилу. */}
            {periodicityReady(draft.recurrence)
              && (openBranch === null || openBranch === branchKind(draft.recurrence)) && (
              <StickyBottomBar>
                <WizardBottomBar>
                  <Button className="w-full" onClick={() => goToStep(4)}>
                    Продолжить
                  </Button>
                </WizardBottomBar>
              </StickyBottomBar>
            )}
          </>
        )}
        {step === 4 && (
          <>
            <PaymentSettingsStep
              draftType={draftType}
              reminderOffsetDays={draft.reminderOffsetDays}
              onReminderOffsetDaysChange={(reminderOffsetDays) =>
                setDraft((prev) => ({ ...prev, reminderOffsetDays }))
              }
              endDate={draft.endDate}
              onEndDateChange={(endDate) => setDraft((prev) => ({ ...prev, endDate }))}
              today={today}
            />
            <StickyBottomBar>
              <WizardBottomBar>
                <Button className="w-full" onClick={() => goToStep(5)}>
                  Далее
                </Button>
              </WizardBottomBar>
            </StickyBottomBar>
          </>
        )}
        {step === 5 && (
          <>
            <AmountStep
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
                  disabled={!wizardStepReady(5, draft)}
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
    goBack(router, ROUTES.propertyPayments(propertyId));
  }

  function navigateBack(): void {
    if (step === 3 && openBranch !== null) {
      setOpenBranch(null);
      return;
    }
    if (step === 1 && categorySearchOpen) {
      setCategorySearchOpen(false);
      setCategoryQuery('');
      return;
    }
    if (step > 1) {
      const prev = (step - 1) as WizardStep;
      setStep(prev);
      // «Назад» — тоже переход: штамп шага едет в черновик (#1055).
      setDraft((prevDraft) => wizardDraftAfterStep(prevDraft, prev));
      return;
    }
    goBack(router, ROUTES.propertyPayments(propertyId));
  }

  function goToStep(next: WizardStep): void {
    setStep(next);
    // Штамп шага в пейлоаде: перезагрузка возвращает на этот же шаг (#1055).
    setDraft((prev) => wizardDraftAfterStep(prev, next));
    if (next !== 3) {
      setOpenBranch(null);
    }
  }


  async function submit(): Promise<void> {
    // Факт «название введено» — из черновика на момент сабмита: в ответе API
    // название уже всегда заполнено (пустое поле замещает лейбл категории).
    const hasTypedTitle = draft.title !== undefined && draft.title.trim().length > 0;
    const command = buildPaymentCreateCommand(draft, {
      autoPay: draftType === 'autopayment',
      resolveTitle: (slug) => paymentCategoryBySlug(slug)?.label,
    });
    if (command === undefined) {
      return;
    }
    try {
      const payment = await createPayment.mutateAsync(command);
      clearDraft();
      notify.scenarios.payments.created();
      setCreated({ payment, hasTypedTitle });
    } catch (error) {
      notify.scenarios.payments.createError(error);
    }
  }
}
