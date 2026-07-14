'use client';

import { useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { Button } from '@/shared/ui/button';
import { LinkButton } from '@/shared/ui/link-button';
import { WizardHeader } from '@/shared/ui/wizard-header';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { type OperationType } from '@/entities/operation/model/types';
import { useCreateOperation } from '@/features/operations/api';
import { useProperties, useProperty } from '@/features/properties/api';
import { useCreateRecurringOperation } from '@/features/recurring-operations/api/hooks';
import { ApiError } from '@/shared/api/errors';
import { SubscriptionReadonlyBanner } from '@/widgets/finance/ui/SubscriptionReadonlyBanner';
import { FinanceErrorState } from '@/widgets/finance/ui/FinanceErrorState';
import { useSubscription } from '@/features/subscription/api/hooks';
import { isSubscriptionReadonly } from '@/features/subscription/lib/is-subscription-readonly';
import { OperationBasicInfoStep } from './OperationBasicInfoStep';
import { OperationCreateWizardLoading } from './OperationCreateWizardLoading';
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
import {
  useOperationCreateDraft,
  type OperationCreateStep,
} from '../lib/use-operation-create-draft';
import styles from './OperationCreateWizard.module.css';

const stepOrder: Exclude<OperationCreateStep, 'success'>[] = ['basic', 'schedule', 'reminder'];

const stepNumber: Record<OperationCreateStep, number> = {
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

function validateBasicInfo(data: BasicInfoData): BasicInfoErrors {
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

  const { data: subscription, isPending: isSubscriptionPending } = useSubscription();
  const readonly = isSubscriptionReadonly(subscription);

  const {
    data: properties,
    isLoading: isPropertiesLoading,
    isError: isPropertiesError,
    isFetching: isPropertiesFetching,
    refetch: refetchProperties,
  } = useProperties();

  const preselectedPropertyQuery = useProperty(propertyId ?? '');
  const isPreselectedArchived =
    Boolean(propertyId) && preselectedPropertyQuery.data?.status === 'archived';

  const { draft, setDraft } = useOperationCreateDraft(type, propertyId);
  const { step, operationType, basicInfo, schedule, reminder } = draft;

  const setStep = (next: OperationCreateStep) =>
    setDraft((prev) => ({ ...prev, step: next }));
  const setBasicInfo = (next: BasicInfoData) =>
    setDraft((prev) => ({ ...prev, basicInfo: next }));
  const setSchedule = (next: ScheduleData) =>
    setDraft((prev) => ({ ...prev, schedule: next }));
  const setReminder = (next: ReminderData) =>
    setDraft((prev) => ({ ...prev, reminder: next }));

  const [basicErrors, setBasicErrors] = useState<BasicInfoErrors>({});
  const [scheduleErrors, setScheduleErrors] = useState<ScheduleErrors>({});
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | undefined>(undefined);

  const createOperation = useCreateOperation();
  const createRecurringOperation = useCreateRecurringOperation();

  const isEmpty = !isPropertiesLoading && !isPropertiesError && properties?.length === 0;
  const isPageLoading =
    isPropertiesLoading ||
    isSubscriptionPending ||
    (Boolean(propertyId) && preselectedPropertyQuery.isPending);

  const handleTypeChange = (nextType: OperationType) => {
    setDraft((prev) => ({
      ...prev,
      operationType: nextType,
      basicInfo: { ...prev.basicInfo, type: nextType, category: undefined },
    }));
  };

  const handleCancel = () => {
    goBack(router, ROUTES.finance);
  };

  const handleBack = () => {
    if (step === 'success') {
      return;
    }

    const currentIndex = stepOrder.indexOf(step);
    if (currentIndex > 0) {
      setStep(stepOrder[currentIndex - 1]);
    } else {
      goBack(router, ROUTES.finance);
    }
  };

  const handleNext = () => {
    setSubmitError(undefined);

    if (step === 'basic') {
      const errors = validateBasicInfo(basicInfo);
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

    const basicValidation = validateBasicInfo(basicInfo);
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
            category_id: category,
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
            category_id: category,
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

  const currentStepNumber = stepNumber[step];
  const isBasicValid = Object.keys(validateBasicInfo(basicInfo)).length === 0;
  const isScheduleValid = Object.keys(validateSchedule(schedule)).length === 0;
  const isNextDisabled =
    readonly || (step === 'basic' && !isBasicValid) || (step === 'schedule' && !isScheduleValid);

  if (step === 'success') {
    return <OperationSuccessScreen type={operationType} propertyId={basicInfo.propertyId} />;
  }

  if (isPageLoading) {
    return (
      <div className={styles.root}>
        <WizardHeader
          title="Создание операции"
          step={currentStepNumber}
          totalSteps={3}
          onBack={handleBack}
          onCancel={handleCancel}
        />
        <OperationCreateWizardLoading />
      </div>
    );
  }

  if (isPropertiesError) {
    return (
      <div className={styles.root}>
        <WizardHeader
          title="Создание операции"
          step={currentStepNumber}
          totalSteps={3}
          onBack={handleBack}
          onCancel={handleCancel}
        />
        <div className={styles.content}>
          <FinanceErrorState onRetry={refetchProperties} isLoading={isPropertiesFetching} />
        </div>
      </div>
    );
  }

  if (isPreselectedArchived && propertyId) {
    return (
      <div className={styles.root}>
        <WizardHeader
          title="Создание операции"
          step={currentStepNumber}
          totalSteps={3}
          onBack={handleBack}
          onCancel={handleCancel}
        />
        <div className={styles.content}>
          <div className={styles.blocked}>
            <h2 className={styles.blockedTitle}>Операция недоступна</h2>
            <p className={styles.blockedText}>
              Объект в архиве. Добавить операцию можно только для объекта в работе
              или на ремонте.
            </p>
            <LinkButton
              href={ROUTES.property(propertyId)}
              variant="primary"
              size="medium"
            >
              К объекту
            </LinkButton>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className={styles.root}>
      <SubscriptionReadonlyBanner />
      <WizardHeader
        title="Создание операции"
        step={currentStepNumber}
        totalSteps={3}
        onBack={handleBack}
        onCancel={handleCancel}
      />

      <div className={styles.content}>
        {step === 'basic' && (
          <OperationBasicInfoStep
            type={operationType}
            data={basicInfo}
            onChange={setBasicInfo}
            onTypeChange={handleTypeChange}
            errors={basicErrors}
            readonly={readonly}
            isEmpty={isEmpty}
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
          disabled={isNextDisabled}
          onClick={step === 'reminder' ? handleSubmit : handleNext}
        >
          {step === 'reminder' ? 'Создать операцию' : 'Далее'}
        </Button>
      </div>
    </div>
  );
}
