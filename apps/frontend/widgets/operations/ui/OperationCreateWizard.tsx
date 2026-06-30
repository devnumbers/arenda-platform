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
import { getCategoriesByType } from '@/entities/operation/lib/categories';
import { useCreateOperation } from '@/features/operations/api';
import { useCreateRecurringOperation } from '@/features/recurring-operations/api/hooks';
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

function validateBasicInfo(data: BasicInfoData, type: OperationType): BasicInfoErrors {
  const errors: BasicInfoErrors = {};
  const amountKopecks = toKopecks(data.amount);

  if (amountKopecks === undefined || amountKopecks <= 0) {
    errors.amount = 'Введите сумму больше 0';
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

  if (data.endDate && data.date && data.endDate < data.date) {
    errors.endDate = 'Дата окончания не может быть раньше даты начала';
  }

  return errors;
}

export function OperationCreateWizard({ type, propertyId }: OperationCreateWizardProps): JSX.Element {
  const router = useRouter();

  const { data: subscription } = useSubscription();
  const readonly = isSubscriptionReadonly(subscription);

  const [operationType, setOperationType] = useState<OperationType>(type);

  const [step, setStep] = useState<Step>('basic');
  const [basicInfo, setBasicInfo] = useState<BasicInfoData>({
    amount: '',
    name: '',
    category: undefined,
    propertyId: propertyId ?? undefined,
    comment: '',
    type,
  });
  const [basicErrors, setBasicErrors] = useState<BasicInfoErrors>({});
  const [schedule, setSchedule] = useState<ScheduleData>({
    frequency: 'once',
    date: undefined,
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

  const handleTypeChange = (nextType: OperationType) => {
    setOperationType(nextType);
    setBasicInfo((prev) => {
      const validCategories = getCategoriesByType(nextType);
      const category = validCategories.some((option) => option.value === prev.category)
        ? prev.category
        : undefined;
      return { ...prev, type: nextType, category };
    });
  };

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
      const errors = validateBasicInfo(basicInfo, operationType);
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

    const basicValidation = validateBasicInfo(basicInfo, operationType);
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

    const selectedPropertyId = basicInfo.propertyId;
    const category = basicInfo.category;
    const operationDate = schedule.date;
    const amountKopecks = toKopecks(basicInfo.amount);

    if (!selectedPropertyId || !category || !operationDate || amountKopecks === undefined) {
      return;
    }

    setIsSubmitting(true);

    try {
      const reminderOffsetDays = reminder.enabled ? reminder.offsetDays : 0;
      const comment = basicInfo.comment?.trim() || undefined;

      if (schedule.frequency === 'once') {
        await createOperation.mutateAsync({
          propertyId: selectedPropertyId,
          data: {
            type: operationType,
            category,
            name: basicInfo.name.trim(),
            amount_kopecks: amountKopecks,
            operation_date: operationDate,
            comment,
            reminder_offset_days: reminderOffsetDays,
          },
        });
      } else {
        await createRecurringOperation.mutateAsync({
          propertyId: selectedPropertyId,
          data: {
            type: operationType,
            category,
            name: basicInfo.name.trim(),
            amount_kopecks: amountKopecks,
            start_date: operationDate,
            end_date: schedule.endDate || undefined,
            comment,
            periodicity: schedule.frequency,
            reminder_offset_days: reminderOffsetDays,
          },
        });
      }

      setStep('success');
    } catch (error: unknown) {
      setSubmitError(formatErrorMessage(error));
    } finally {
      setIsSubmitting(false);
    }
  };

  if (step === 'success') {
    return <OperationSuccessScreen type={operationType} propertyId={basicInfo.propertyId} />;
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
            type={operationType}
            data={basicInfo}
            onChange={setBasicInfo}
            onTypeChange={handleTypeChange}
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
          <OperationReminderStep
            data={reminder}
            onChange={setReminder}
            readonly={readonly}
          />
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
