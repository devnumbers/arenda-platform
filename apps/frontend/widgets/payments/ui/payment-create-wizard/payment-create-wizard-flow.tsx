'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { notify } from '@/shared/lib/notifications';
import {
  Button,
  IconButton,
  PageContent,
  StepsChip,
  StickyBottomBar,
  TopNav,
} from '@/shared/ui/design';
import type { IsoDate, Payment } from '@/entities/payment';
import { clientTodayIso, formatDayMonthWithYear } from '@/entities/payment';
import {
  buildPaymentCreateCommand,
  periodicityReady,
  useCreatePayment,
  usePaymentWizardDraft,
  WIZARD_TOTAL_STEPS,
  wizardStepReady,
  type PaymentDraftType,
  type PaymentWizardDraft,
  type PeriodicityBranch,
  type WizardStep,
} from '@/features/payments';
import { paymentCategoryBySlug } from '@/features/payment-categories';
import { firstOccurrencePreview } from '../../lib/first-occurrence';
import { AmountStep } from './amount-step';
import { CategoryStep } from './category-step';
import { EndDateStep } from './end-date-step';
import { PeriodicityStep } from './periodicity-step';
import { TitleStep } from './title-step';
import { WizardBottomBar } from './wizard-chrome';
import { WizardSuccess } from './wizard-success';

/**
 * Поток шагов визарда (клиентское состояние на одном маршруте #464):
 * монтируется после гидрации черновика и загрузки объекта родителем,
 * восстанавливается на первый незавершённый шаг; черновик живёт в
 * localStorage per объект+тип (история 10 спеки #453).
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
  const [created, setCreated] = useState<Payment | null>(null);
  const [step, setStep] = useState<WizardStep>(() => initialStep(draft));
  const [openBranch, setOpenBranch] = useState<PeriodicityBranch | null>(null);

  const today: IsoDate = clientTodayIso();

  if (created !== null) {
    return (
      <>
        <TopNav />
        <PageContent>
          <div className="pt-16">
            <WizardSuccess created={created} draftType={draftType} onClose={closeAfterCreation} />
          </div>
        </PageContent>
      </>
    );
  }

  return (
    <>
      <TopNav
        leading={<IconButton icon={<ArrowLeft />} label="Назад" onClick={navigateBack} />}
      >
        <StepsChip step={step} total={WIZARD_TOTAL_STEPS} size="m" />
      </TopNav>
      <PageContent>
        {step === 1 && (
          <CategoryStep
            selectedSlug={draft.categorySlug}
            onSelect={(slug) => {
              setDraft((prev) => ({ ...prev, categorySlug: slug }));
              goToStep(2);
            }}
          />
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
                  Далее
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
              today={today}
            />
            {(openBranch !== null || periodicityReady(draft.recurrence)) && (
              <StickyBottomBar>
                <WizardBottomBar>
                  {previewLine()}
                  <Button
                    className="w-full"
                    disabled={!periodicityReady(draft.recurrence)}
                    onClick={() => goToStep(4)}
                  >
                    Далее
                  </Button>
                </WizardBottomBar>
              </StickyBottomBar>
            )}
          </>
        )}
        {step === 4 && (
          <>
            <EndDateStep
              endDate={draft.endDate}
              onEndDateChange={(endDate) => setDraft((prev) => ({ ...prev, endDate }))}
              today={today}
            />
            <StickyBottomBar>
              <WizardBottomBar>
                {previewLine()}
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
              paymentForm={draft.paymentForm}
              onPaymentFormChange={(paymentForm) =>
                setDraft((prev) => ({ ...prev, paymentForm }))
              }
            />
            <StickyBottomBar>
              <WizardBottomBar>
                {previewLine()}
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
    if (step > 1) {
      setStep((prev) => ((prev - 1) as WizardStep));
      return;
    }
    goBack(router, ROUTES.propertyPayments(propertyId));
  }

  function goToStep(next: WizardStep): void {
    setStep(next);
    if (next !== 3) {
      setOpenBranch(null);
    }
  }

  /** Превью первого вхождения над кнопкой шага (клиентский порт, история 9). */
  function previewLine(): JSX.Element | null {
    const recurrence = draft.recurrence;
    if (recurrence === undefined || !periodicityReady(recurrence)) return null;
    const first = firstOccurrencePreview(recurrence, today, draft.endDate);
    return (
      <p className="text-center text-sm text-content-secondary">
        Первое вхождение — {first !== null ? formatDayMonthWithYear(first, today) : 'нет'}
      </p>
    );
  }

  async function submit(): Promise<void> {
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
      setCreated(payment);
    } catch (error) {
      notify.scenarios.payments.createError(error);
    }
  }
}

/** Восстановление черновика: первый незавершённый шаг (2 и 4 необязательны). */
function initialStep(draft: PaymentWizardDraft): WizardStep {
  if (!wizardStepReady(1, draft)) return 1;
  if (!wizardStepReady(3, draft)) return 3;
  return 5;
}
