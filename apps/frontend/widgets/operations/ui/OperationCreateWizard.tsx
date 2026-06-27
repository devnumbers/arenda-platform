'use client';

import { useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { toast } from 'react-toastify';
import { ArrowLeft, Cancel } from '@/shared/assets/icons';
import { Button } from '@/shared/ui/button';
import { IconButton } from '@/shared/ui/icon-button';
import { ROUTES } from '@/shared/config/routes';
import {
  expenseCategories,
  incomeCategories,
  type OperationType,
} from '@/entities/operation/model/types';
import {
  useCreateOperation,
  useCreateOperationReminder,
  useCreateRecurringOperation,
  useCreateRecurringOperationReminder,
} from '@/features/operations/api';
import { ApiError } from '@/shared/api/errors';
import { SubscriptionReadonlyBanner } from '@/widgets/finance/ui/SubscriptionReadonlyBanner';
import { useSubscription } from '@/features/subscription/api/hooks';
import { isSubscriptionReadonly } from '@/features/subscription/lib/is-subscription-readonly';
import { OperationBasicInfoStep } from './OperationBasicInfoStep';
import { OperationReminderStep } from './OperationReminderStep';
import { OperationScheduleStep } from './OperationScheduleStep';
import { OperationSuccessScreen } from './OperationSuccessScreen';
import {
  toKopecks,
  type BasicInfoData,
  type BasicInfoErrors,
  type ReminderData,
  type ScheduleData,
  type ScheduleErrors,
} from '../model/types';
import styles from './OperationCreateWizard.module.css';

type Step = 'basic' | 'schedule' | 'reminder' | 'success';

const stepOrder: Exclude<Step, 'success'>[] = ['basic', 'schedule', 'reminder'];

const stepNumber: Record<Step, number> = {
  basic: 1,
  schedule: 2,
  reminder: 3,
  success: 0,
};

export type OperationCreateWizardProps = {
  readonly type: OperationType;
  readonly propertyId?: string;
};

function formatErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    return error.detail;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return 'Не удалось создать операцию. Попробуйте ещё раз.';
}

function parseLocalDate(value: string): Date {
  const [year, month, day] = value.split('-').map(Number);
  return new Date(year, month - 1, day);
}

function formatDate(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

function addDays(date: Date, days: number): Date {
  const result = new Date(date);
  result.setDate(result.getDate() + days);
  return result;
}

function subtractDaysFromDateString(dateString: string, days: number): string {
  return formatDate(addDays(parseLocalDate(dateString), -days));
}

function parseUtcDate(value: string): Date {
  const [year, month, day] = value.split('-').map(Number);
  return new Date(Date.UTC(year, month - 1, day));
}

function formatUtcDate(date: Date): string {
  const year = date.getUTCFullYear();
  const month = String(date.getUTCMonth() + 1).padStart(2, '0');
  const day = String(date.getUTCDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

function startOfTodayUtc(): Date {
  const now = new Date();
  return new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()));
}

function lastDayOfUtcMonth(year: number, month: number): number {
  return new Date(Date.UTC(year, month + 1, 0)).getUTCDate();
}

function addUtcMonths(date: Date, months: number): Date {
  const result = new Date(date);
  result.setUTCMonth(result.getUTCMonth() + months);
  return result;
}

function nextPaymentDateMonthly(current: Date, paymentDay: number): Date {
  const year = current.getUTCFullYear();
  const month = current.getUTCMonth();
  const lastDay = lastDayOfUtcMonth(year, month + 1);
  const day = Math.min(paymentDay, lastDay);
  return new Date(Date.UTC(year, month + 1, day));
}

function nextPaymentDateYearly(current: Date, paymentDay: number): Date {
  const year = current.getUTCFullYear() + 1;
  const month = current.getUTCMonth();
  const lastDay = lastDayOfUtcMonth(year, month);
  const day = Math.min(paymentDay, lastDay);
  return new Date(Date.UTC(year, month, day));
}

function getEarliestFutureOperationDate(schedule: ScheduleData): string | null {
  if (!schedule.date || schedule.frequency === 'once') {
    return null;
  }

  const today = startOfTodayUtc();
  const startDate = parseUtcDate(schedule.date);
  const endDate = schedule.endDate ? parseUtcDate(schedule.endDate) : undefined;
  const paymentDay = schedule.paymentDay ?? startDate.getUTCDate();

  const windowEnd = addUtcMonths(today, 12);
  let current = new Date(startDate);
  let first = true;

  for (let i = 0; i < 37; i++) {
    if (!first) {
      if (schedule.frequency === 'yearly') {
        current = nextPaymentDateYearly(current, paymentDay);
      } else {
        current = nextPaymentDateMonthly(current, paymentDay);
      }
    }
    first = false;

    if (endDate && current > endDate) {
      break;
    }
    if (current > windowEnd) {
      break;
    }

    if (current >= today) {
      return formatUtcDate(current);
    }
  }

  return null;
}

function validateBasicInfo(data: BasicInfoData, type: OperationType): BasicInfoErrors {
  const errors: BasicInfoErrors = {};
  const amountKopecks = toKopecks(data.amount);

  if (amountKopecks === undefined || amountKopecks < 0) {
    errors.amount = 'Введите сумму';
  }

  const name = data.name.trim();
  if (name === '') {
    errors.name = 'Введите название операции';
  } else if (name.length > 50) {
    errors.name = 'Название не должно превышать 50 символов';
  }

  if (!data.category) {
    errors.category = 'Выберите категорию';
  } else {
    const validOptions = type === 'income' ? incomeCategories : expenseCategories;
    if (!validOptions.some((option) => option.value === data.category)) {
      errors.category = 'Выберите категорию';
    }
  }

  if (!data.propertyId) {
    errors.property = 'Выберите объект';
  }

  if (data.comment && data.comment.length > 500) {
    errors.comment = 'Комментарий не должен превышать 500 символов';
  }

  return errors;
}

function validateSchedule(data: ScheduleData): ScheduleErrors {
  const errors: ScheduleErrors = {};

  if (!data.date) {
    errors.date = 'Выберите дату';
  }

  if (data.frequency !== 'once') {
    if (data.paymentDay === undefined) {
      errors.paymentDay = 'Укажите день операции';
    } else if (data.paymentDay < 1 || data.paymentDay > 31) {
      errors.paymentDay = 'День операции должен быть от 1 до 31';
    }
  }

  if (data.endDate && data.date && data.endDate < data.date) {
    errors.endDate = 'Дата окончания не может быть раньше даты начала';
  }

  return errors;
}

export function OperationCreateWizard({ type, propertyId }: OperationCreateWizardProps): JSX.Element {
  const router = useRouter();

  const { data: subscription } = useSubscription();
  const readonly = isSubscriptionReadonly(subscription);

  const [step, setStep] = useState<Step>('basic');
  const [basicInfo, setBasicInfo] = useState<BasicInfoData>({
    amount: '',
    name: '',
    category: undefined,
    propertyId: propertyId ?? undefined,
    comment: '',
  });
  const [basicErrors, setBasicErrors] = useState<BasicInfoErrors>({});
  const [schedule, setSchedule] = useState<ScheduleData>({
    frequency: 'once',
    date: undefined,
    paymentDay: undefined,
    endDate: undefined,
  });
  const [scheduleErrors, setScheduleErrors] = useState<ScheduleErrors>({});
  const [reminder, setReminder] = useState<ReminderData>({
    enabled: false,
    offsetDays: 1,
  });
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | undefined>(undefined);

  const createOperation = useCreateOperation();
  const createRecurringOperation = useCreateRecurringOperation();
  const createOperationReminder = useCreateOperationReminder();
  const createRecurringOperationReminder = useCreateRecurringOperationReminder();

  const handleCancel = () => {
    router.push(ROUTES.finance);
  };

  const handleBack = () => {
    if (step === 'success') {
      return;
    }

    const currentIndex = stepOrder.indexOf(step);
    if (currentIndex > 0) {
      setStep(stepOrder[currentIndex - 1]);
    } else {
      router.push(ROUTES.finance);
    }
  };

  const handleNext = () => {
    setSubmitError(undefined);

    if (step === 'basic') {
      const errors = validateBasicInfo(basicInfo, type);
      setBasicErrors(errors);
      if (Object.keys(errors).length === 0) {
        setStep('schedule');
      }
      return;
    }

    if (step === 'schedule') {
      const errors = validateSchedule(schedule);
      setScheduleErrors(errors);
      if (Object.keys(errors).length === 0) {
        setStep('reminder');
      }
    }
  };

  const handleSubmit = async () => {
    setSubmitError(undefined);

    const basicValidation = validateBasicInfo(basicInfo, type);
    const scheduleValidation = validateSchedule(schedule);
    setBasicErrors(basicValidation);
    setScheduleErrors(scheduleValidation);

    if (Object.keys(basicValidation).length > 0) {
      setStep('basic');
      return;
    }

    if (Object.keys(scheduleValidation).length > 0) {
      setStep('schedule');
      return;
    }

    const propertyId = basicInfo.propertyId;
    const category = basicInfo.category;
    const operationDate = schedule.date;
    const amountKopecks = toKopecks(basicInfo.amount);

    if (!propertyId || !category || !operationDate || amountKopecks === undefined) {
      return;
    }

    setIsSubmitting(true);

    try {
      if (schedule.frequency === 'once') {
        const operation = await createOperation.mutateAsync({
          propertyId,
          data: {
            type,
            category,
            name: basicInfo.name.trim(),
            amount_kopecks: amountKopecks,
            operation_date: operationDate,
            comment: basicInfo.comment?.trim() || undefined,
          },
        });

        if (reminder.enabled) {
          try {
            await createOperationReminder.mutateAsync({
              propertyId,
              operationId: operation.id,
              data: {
                reminder_date: subtractDaysFromDateString(operationDate, reminder.offsetDays),
              },
            });
          } catch {
            toast.success('Операция создана, но не удалось добавить напоминание');
          }
        }
      } else {
        const recurringOperation = await createRecurringOperation.mutateAsync({
          propertyId,
          data: {
            type,
            category,
            name: basicInfo.name.trim(),
            amount_kopecks: amountKopecks,
            start_date: operationDate,
            payment_day: schedule.paymentDay ?? 1,
            end_date: schedule.endDate || undefined,
            comment: basicInfo.comment?.trim() || undefined,
            periodicity: schedule.frequency,
          },
        });

        if (reminder.enabled) {
          const earliestDate = getEarliestFutureOperationDate(schedule);
          if (earliestDate) {
            try {
              await createRecurringOperationReminder.mutateAsync({
                propertyId,
                recurringOperationId: recurringOperation.id,
                data: {
                  reminder_date: subtractDaysFromDateString(earliestDate, reminder.offsetDays),
                },
              });
            } catch {
              toast.success('Операция создана, но не удалось добавить напоминание');
            }
          }
        }
      }

      setStep('success');
    } catch (error: unknown) {
      setSubmitError(formatErrorMessage(error));
    } finally {
      setIsSubmitting(false);
    }
  };

  if (step === 'success') {
    return <OperationSuccessScreen type={type} propertyId={basicInfo.propertyId} />;
  }

  const currentStepNumber = stepNumber[step];

  return (
    <div className={styles.root}>
      <SubscriptionReadonlyBanner />
      <header className={styles.header}>
        <div className={styles.topRow}>
          <IconButton
            variant="icon-black"
            size="small"
            icon={<ArrowLeft />}
            aria-label="Назад"
            onClick={handleBack}
            className={styles.iconButton}
          />
          <h1 className={styles.title}>Создание операции</h1>
          <IconButton
            variant="icon-black"
            size="small"
            icon={<Cancel />}
            aria-label="Отменить"
            onClick={handleCancel}
            className={styles.iconButton}
          />
        </div>
        <div className={styles.progressRow}>
          <span className={styles.badge}>{currentStepNumber} из 3</span>
          <div
            className={styles.progressBar}
            role="progressbar"
            aria-label={`Шаг ${currentStepNumber} из 3`}
            aria-valuenow={currentStepNumber}
            aria-valuemin={1}
            aria-valuemax={3}
          >
            <div className={`${styles.segment} ${currentStepNumber >= 1 ? styles.active : ''}`} />
            <div className={`${styles.segment} ${currentStepNumber >= 2 ? styles.active : ''}`} />
            <div className={`${styles.segment} ${currentStepNumber >= 3 ? styles.active : ''}`} />
          </div>
        </div>
      </header>

      <div className={styles.content}>
        {step === 'basic' && (
          <OperationBasicInfoStep
            type={type}
            data={basicInfo}
            onChange={setBasicInfo}
            errors={basicErrors}
            readonly={readonly}
          />
        )}
        {step === 'schedule' && (
          <OperationScheduleStep
            data={schedule}
            onChange={setSchedule}
            errors={scheduleErrors}
            readonly={readonly}
          />
        )}
        {step === 'reminder' && (
          <OperationReminderStep data={reminder} onChange={setReminder} />
        )}
      </div>

      <div className={styles.footer}>
        {submitError && <p className={styles.error}>{submitError}</p>}
        <Button
          variant="primary"
          size="large"
          fullWidth
          loading={isSubmitting}
          disabled={readonly}
          onClick={step === 'reminder' ? handleSubmit : handleNext}
        >
          {step === 'reminder' ? 'Создать операцию' : 'Далее'}
        </Button>
      </div>
    </div>
  );
}
