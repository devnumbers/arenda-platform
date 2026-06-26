'use client';

import { useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
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
};

function formatErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    return error.detail;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return 'Не удалось создать платёж. Попробуйте ещё раз.';
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

function startOfToday(): Date {
  const now = new Date();
  return new Date(now.getFullYear(), now.getMonth(), now.getDate());
}

function addDays(date: Date, days: number): Date {
  const result = new Date(date);
  result.setDate(result.getDate() + days);
  return result;
}

function daysInMonth(year: number, month: number): number {
  return new Date(year, month + 1, 0).getDate();
}

function subtractDaysFromDateString(dateString: string, days: number): string {
  return formatDate(addDays(parseLocalDate(dateString), -days));
}

function getEarliestFutureOperationDate(schedule: ScheduleData): string | undefined {
  if (!schedule.date) {
    return undefined;
  }

  const today = startOfToday();
  const startDate = parseLocalDate(schedule.date);

  if (startDate >= today) {
    return schedule.date;
  }

  if (schedule.frequency === 'yearly') {
    const date = new Date(startDate);
    while (date < today) {
      date.setFullYear(date.getFullYear() + 1);
    }
    return formatDate(date);
  }

  const paymentDay = schedule.paymentDay ?? startDate.getDate();
  let year = today.getFullYear();
  let month = today.getMonth();
  let day = Math.min(paymentDay, daysInMonth(year, month));
  let candidate = new Date(year, month, day);

  if (candidate < today) {
    month += 1;
    if (month > 11) {
      month = 0;
      year += 1;
    }
    day = Math.min(paymentDay, daysInMonth(year, month));
    candidate = new Date(year, month, day);
  }

  return formatDate(candidate);
}

function validateBasicInfo(data: BasicInfoData, type: OperationType): BasicInfoErrors {
  const errors: BasicInfoErrors = {};
  const amountKopecks = toKopecks(data.amount);

  if (amountKopecks === undefined || amountKopecks <= 0) {
    errors.amount = 'Введите сумму больше 0';
  }

  const name = data.name.trim();
  if (name === '') {
    errors.name = 'Введите название платежа';
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
      errors.paymentDay = 'Укажите день платежа';
    } else if (data.paymentDay < 1 || data.paymentDay > 31) {
      errors.paymentDay = 'День платежа должен быть от 1 до 31';
    }
  }

  if (data.endDate && data.date && data.endDate < data.date) {
    errors.endDate = 'Дата окончания не может быть раньше даты начала';
  }

  return errors;
}

export function OperationCreateWizard({ type }: OperationCreateWizardProps): JSX.Element {
  const router = useRouter();

  const [step, setStep] = useState<Step>('basic');
  const [basicInfo, setBasicInfo] = useState<BasicInfoData>({
    amount: '',
    name: '',
    category: undefined,
    propertyId: undefined,
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
          await createOperationReminder.mutateAsync({
            propertyId,
            operationId: operation.id,
            data: {
              reminder_date: subtractDaysFromDateString(operationDate, reminder.offsetDays),
            },
          });
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
            await createRecurringOperationReminder.mutateAsync({
              propertyId,
              recurringOperationId: recurringOperation.id,
              data: {
                reminder_date: subtractDaysFromDateString(earliestDate, reminder.offsetDays),
              },
            });
          }
        }
      }

      setStep('success');
    } catch (error: unknown) {
      console.error('Failed to create operation', error);
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
          <h1 className={styles.title}>Создание платежа</h1>
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
          />
        )}
        {step === 'schedule' && (
          <OperationScheduleStep
            data={schedule}
            onChange={setSchedule}
            errors={scheduleErrors}
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
          onClick={step === 'reminder' ? handleSubmit : handleNext}
        >
          {step === 'reminder' ? 'Создать платёж' : 'Далее'}
        </Button>
      </div>
    </div>
  );
}
