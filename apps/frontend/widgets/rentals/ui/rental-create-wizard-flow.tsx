'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft, Cancel } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { notify } from '@/shared/lib/notifications';
import { dateToIsoLocal } from '@/shared/lib/calendar';
import { type IsoDate, type PaymentReminderOffset } from '@/entities/payment';
import type { Rental } from '@/entities/rental';
import {
  useCreateRental,
  useRentalWizardSession,
  wizardStepReady,
  draftAfterStartChange,
  WIZARD_TOTAL_STEPS,
  RENTAL_REMINDER_DEFAULT,
  buildRentalCreateCommand,
  type RentalWizardDraft,
  type RentalWizardStep,
} from '@/features/rentals';
import {
  Button,
  IconButton,
  PageContent,
  StepsChip,
  StickyBottomBar,
  TopNav,
  useTabBarSuppression,
} from '@/shared/ui/design';
import { AmountDayStep } from './amount-day-step';
import { ContactStep } from './contact-step';
import { ConditionsStep } from './conditions-step';
import { SettingsStep } from './settings-step';
import { RentalWizardSuccess } from './rental-wizard-success';
import { WizardBottomBar } from './wizard-chrome';

/**
 * Поток шагов визарда создания аренды (#530, клиентское состояние на одном
 * маршруте): монтируется после загрузки объекта, восстанавливается на первый
 * незавершённый шаг. Состояние шагов живёт в носителе сессии per объект
 * (rental-wizard-session, карта #1052 D3): уход в ветви шага контакта —
 * экран выбора арендатора (#807) и создание контакта (#509) — помечает
 * сессию живой до push, и круговой маршрут возвращает на тот же шаг с теми
 * же полями; любой другой уход из визарда, перезагрузка и закрытие дают
 * чистый лист.
 */

export type RentalCreateWizardFlowProps = {
  readonly propertyId: string;
};

export function RentalCreateWizardFlow({
  propertyId,
}: RentalCreateWizardFlowProps): JSX.Element {
  const router = useRouter();
  const createRental = useCreateRental(propertyId);
  // Владелец сессии: хук ведёт жизненный цикл носителя — открытие гасит
  // keep-alive, закрытие без флага чистит носитель.
  const { draft, setDraft, clearDraft, keepAlive } = useRentalWizardSession(propertyId);
  // Визард — экран создания: футер глушится на всех шагах.
  useTabBarSuppression();
  const [created, setCreated] = useState<Rental | null>(null);
  const today: IsoDate = dateToIsoLocal(new Date());
  const [step, setStep] = useState<RentalWizardStep>(() => initialStep(draft, today));

  if (created !== null) {
    return (
      <>
        <TopNav
          leading={
            <IconButton icon={<Cancel />} label="Закрыть" onClick={closeWizard} />
          }
        />
        <PageContent>
          <div className="pt-16">
            <RentalWizardSuccess created={created} onClose={closeWizard} />
          </div>
        </PageContent>
      </>
    );
  }

  return (
    <>
      {/* Хром шагов по макету (TopNav 1270:46906/46823): шаг 1 — крестик
          «Закрыть» слева (назад внутри визарда нет); шаги 2–4 — стрелка
          «Назад» слева и крестик «Закрыть» справа — выход из визарда
          целиком с любого шага (носитель сессии чистится — следующий вход
          откроет чистый визард). */}
      <TopNav
        leading={
          step === 1 ? (
            <IconButton icon={<Cancel />} label="Закрыть" onClick={closeWizard} />
          ) : (
            <IconButton icon={<ArrowLeft />} label="Назад" onClick={navigateBack} />
          )
        }
        trailing={
          step > 1 ? (
            <IconButton icon={<Cancel />} label="Закрыть" onClick={closeWizard} />
          ) : undefined
        }
      >
        <StepsChip step={step} total={WIZARD_TOTAL_STEPS} size="m" />
      </TopNav>
      <PageContent className="pt-0">
        {step === 1 && (
          <>
            <AmountDayStep
              amountKopecks={draft.amountKopecks}
              onAmountChange={(amountKopecks) => setDraft((prev) => ({ ...prev, amountKopecks }))}
              paymentDay={draft.paymentDay}
              onPaymentDayChange={(paymentDay) => setDraft((prev) => ({ ...prev, paymentDay }))}
            />
            {/* Кнопка продолжения скрыта, пока шаг не готов (решение
                владельца 2026-09-05: скрытие вместо дизейбла); на планшете
                тянется с шитом во всю ширину, как хедер. */}
            {wizardStepReady(1, draft, today) && (
              <StickyBottomBar>
                <WizardBottomBar>
                  <Button className="w-full" onClick={() => goToStep(2)}>
                    Продолжить
                  </Button>
                </WizardBottomBar>
              </StickyBottomBar>
            )}
          </>
        )}
        {step === 2 && (
          <>
            <ConditionsStep
              startDate={draft.startDate}
              onStartDateChange={(startDate) => setDraft((prev) => draftAfterStartChange(prev, startDate))}
              plannedEndDate={draft.plannedEndDate}
              onPlannedEndDateChange={(plannedEndDate) =>
                setDraft((prev) => ({ ...prev, plannedEndDate }))
              }
              utilities={draft.utilities}
              onUtilitiesChange={(utilities) => setDraft((prev) => ({ ...prev, utilities }))}
              depositKopecks={draft.depositKopecks}
              onDepositChange={(depositKopecks) =>
                setDraft((prev) => ({ ...prev, depositKopecks }))
              }
              commissionKopecks={draft.commissionKopecks}
              onCommissionChange={(commissionKopecks) =>
                setDraft((prev) => ({ ...prev, commissionKopecks }))
              }
              today={today}
            />
            {wizardStepReady(2, draft, today) && (
              <StickyBottomBar>
                <WizardBottomBar>
                  <Button className="w-full" onClick={() => goToStep(3)}>
                    Далее
                  </Button>
                </WizardBottomBar>
              </StickyBottomBar>
            )}
          </>
        )}
        {step === 3 && (
          <>
            <SettingsStep
              autoPay={draft.autoPay ?? false}
              onAutoPayChange={(autoPay) => setDraft((prev) => ({ ...prev, autoPay }))}
              reminderOffsetDays={draft.reminderOffsetDays ?? RENTAL_REMINDER_DEFAULT}
              onReminderOffsetChange={(reminderOffsetDays: PaymentReminderOffset) =>
                setDraft((prev) => ({ ...prev, reminderOffsetDays }))
              }
            />
            {/* Шаг всегда готов (тумблер с дефолтом) — кнопка видна всегда. */}
            <StickyBottomBar>
              <WizardBottomBar>
                <Button className="w-full" onClick={() => goToStep(4)}>
                  Продолжить
                </Button>
              </WizardBottomBar>
            </StickyBottomBar>
          </>
        )}
        {step === 4 && (
          <>
            <ContactStep
              propertyId={propertyId}
              contactId={draft.contactId}
              onContactChange={(contactId) => setDraft((prev) => ({ ...prev, contactId }))}
              onSubRouteOpen={keepAlive}
            />
            {/* Шаг всегда готов (арендатор необязателен) — кнопка видна
                всегда; валидность целиком проверяет сабмит. */}
            <StickyBottomBar>
              <WizardBottomBar>
                <Button
                  className="w-full"
                  loading={createRental.isPending}
                  onClick={() => void submit()}
                >
                  Создать аренду
                </Button>
              </WizardBottomBar>
            </StickyBottomBar>
          </>
        )}
      </PageContent>
    </>
  );

  /** Выход из визарда (крестик на любом шаге, «Хорошо» на экране успеха):
   * история назад, при пустой истории — фолбэк goBack'а на объект,
   * носитель сессии чистится при размонтировании. */
  function closeWizard(): void {
    goBack(router, ROUTES.property(propertyId));
  }

  function navigateBack(): void {
    if (step > 1) {
      const previous: RentalWizardStep = step === 4 ? 3 : step === 3 ? 2 : 1;
      setStep(previous);
      return;
    }
    goBack(router, ROUTES.property(propertyId));
  }

  function goToStep(next: RentalWizardStep): void {
    setStep(next);
  }

  async function submit(): Promise<void> {
    const command = buildRentalCreateCommand(draft, today);
    if (command === undefined) {
      return;
    }
    try {
      const rental = await createRental.mutateAsync(command);
      clearDraft();
      notify.scenarios.rentals.created();
      setCreated(rental);
    } catch (error: unknown) {
      notify.scenarios.rentals.createError(error);
    }
  }
}

/** Первый незавершённый шаг при восстановлении черновика (после обмена
 * шагов 3 — настройки и 4 — арендатор всегда готовы). */
function initialStep(draft: RentalWizardDraft, today: IsoDate): RentalWizardStep {
  if (!wizardStepReady(1, draft, today)) return 1;
  if (!wizardStepReady(2, draft, today)) return 2;
  return 4;
}
