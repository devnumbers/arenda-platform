'use client';

import { useState, type ChangeEvent, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { useId } from 'react';
import clsx from 'clsx';
import { Button } from '@/shared/ui/button';
import { TextField } from '@/shared/ui/text-field';
import { DateSelect } from '@/shared/ui/date-select';
import { WizardHeader } from '@/shared/ui/wizard-header';
import { Bell } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { buildTriggerAt, extractLocalDate, extractLocalTime } from '@/shared/lib/datetime';
import { notify } from '@/shared/lib/notifications';
import { PropertySelect } from '@/features/properties';
import {
  useCreateFreeReminder,
  useUpdateFreeReminder,
} from '@/features/free-reminders';
import {
  PERIODICITY_OPTIONS,
  type FreeReminder,
  type FreeReminderPeriodicity,
} from '@/features/free-reminders';
import styles from './FreeReminderWizard.module.css';

type WizardStep = 1 | 2;

export type FreeReminderWizardProps = {
  readonly mode: 'create' | 'edit';
  readonly propertyId?: string;
  readonly reminder?: FreeReminder;
  readonly onDone?: () => void;
};

type Draft = {
  readonly title: string;
  readonly propertyId: string | undefined;
  readonly date: string;
  readonly time: string;
  readonly periodicity: FreeReminderPeriodicity;
};

type Errors = {
  title?: string;
  property?: string;
  date?: string;
  time?: string;
};

function buildInitialDraft(
  mode: 'create' | 'edit',
  propertyId: string | undefined,
  reminder: FreeReminder | undefined,
): Draft {
  if (mode === 'edit' && reminder) {
    return {
      title: reminder.title,
      propertyId: reminder.propertyId,
      date: extractLocalDate(reminder.triggerAt),
      time: extractLocalTime(reminder.triggerAt),
      periodicity: reminder.periodicity,
    };
  }

  return {
    title: '',
    propertyId,
    date: '',
    time: '10:00',
    periodicity: 'once',
  };
}

function validateBasic(draft: Draft): Errors {
  const errors: Errors = {};
  const title = draft.title.trim();
  if (title === '') {
    errors.title = 'Введите название';
  } else if (title.length > 50) {
    errors.title = 'Название не должно превышать 50 символов';
  }
  if (!draft.propertyId) {
    errors.property = 'Выберите объект';
  }
  return errors;
}

function validateSchedule(draft: Draft): Errors {
  const errors: Errors = {};
  if (!draft.date) {
    errors.date = 'Выберите дату';
  }
  if (!draft.time) {
    errors.time = 'Выберите время';
  }
  return errors;
}

export function FreeReminderWizard({
  mode,
  propertyId,
  reminder,
  onDone,
}: FreeReminderWizardProps): JSX.Element {
  const router = useRouter();
  const timeInputId = useId();

  const [step, setStep] = useState<WizardStep>(1);
  const [draft, setDraft] = useState<Draft>(() =>
    buildInitialDraft(mode, propertyId, reminder),
  );
  const [basicErrors, setBasicErrors] = useState<Errors>({});
  const [scheduleErrors, setScheduleErrors] = useState<Errors>({});

  const createMutation = useCreateFreeReminder();
  const updateMutation = useUpdateFreeReminder();
  const isSubmitting = createMutation.isPending || updateMutation.isPending;

  const exitWizard = (): void => {
    if (mode === 'edit') {
      onDone?.();
      return;
    }
    goBack(router, ROUTES.properties);
  };

  const handleBack = (): void => {
    if (step === 2) {
      setStep(1);
      return;
    }
    exitWizard();
  };

  const handleTitleChange = (event: ChangeEvent<HTMLInputElement>): void => {
    const title = event.currentTarget.value;
    setDraft((prev) => ({ ...prev, title }));
  };

  const handleTimeChange = (event: ChangeEvent<HTMLInputElement>): void => {
    const time = event.currentTarget.value;
    setDraft((prev) => ({ ...prev, time }));
  };

  const handleNext = (): void => {
    const errors = validateBasic(draft);
    setBasicErrors(errors);
    if (Object.keys(errors).length === 0) {
      setStep(2);
    }
  };

  const handleSubmit = async (): Promise<void> => {
    const basicValidation = validateBasic(draft);
    const scheduleValidation = validateSchedule(draft);
    setBasicErrors(basicValidation);
    setScheduleErrors(scheduleValidation);

    if (Object.keys(basicValidation).length > 0) {
      setStep(1);
      return;
    }
    if (Object.keys(scheduleValidation).length > 0) {
      return;
    }

    const triggerAt = buildTriggerAt(draft.date, draft.time);

    try {
      if (mode === 'edit' && reminder) {
        await updateMutation.mutateAsync({
          id: reminder.id,
          data: {
            title: draft.title.trim(),
            triggerAt,
            periodicity: draft.periodicity,
          },
        });
        onDone?.();
      } else {
        const created = await createMutation.mutateAsync({
          propertyId: draft.propertyId!,
          data: {
            title: draft.title.trim(),
            triggerAt,
            periodicity: draft.periodicity,
          },
        });
        router.replace(ROUTES.freeReminder(created.id));
      }
    } catch (error: unknown) {
      if (mode === 'edit') {
        notify.scenarios.freeReminders.updateError(error);
      } else {
        notify.scenarios.freeReminders.createError(error);
      }
    }
  };

  const isBasicValid = Object.keys(validateBasic(draft)).length === 0;
  const isNextDisabled = step === 1 && !isBasicValid;

  return (
    <div className={styles.root}>
      <WizardHeader
        title={mode === 'edit' ? 'Изменить напоминание' : 'Новое напоминание'}
        step={step}
        totalSteps={2}
        onBack={handleBack}
        onCancel={exitWizard}
      />

      <div className={styles.content}>
        {step === 1 && (
          <section className={styles.stepFields}>
            <h2 className={styles.stepTitle}>Основное</h2>
            <TextField
              label="Название"
              placeholder="Например, оплатить интернет"
              required
              maxLength={50}
              showCounter
              fullWidth
              value={draft.title}
              onChange={handleTitleChange}
              error={basicErrors.title}
            />
            <PropertySelect
              value={draft.propertyId}
              onChange={(value) =>
                setDraft((prev) => ({ ...prev, propertyId: value }))
              }
              error={basicErrors.property}
              disabled={mode === 'edit'}
            />
          </section>
        )}

        {step === 2 && (
          <section className={styles.stepFields}>
            <h2 className={styles.stepTitle}>Расписание</h2>
            <div className={styles.datetimeRow}>
              <DateSelect
                label="Дата"
                value={draft.date || undefined}
                onChange={(value) =>
                  setDraft((prev) => ({ ...prev, date: value ?? '' }))
                }
                required
                error={scheduleErrors.date}
              />
              <div className={styles.timeField}>
                <label className={styles.timeLabel} htmlFor={timeInputId}>
                  Время<span className={styles.required}>*</span>
                </label>
                <input
                  id={timeInputId}
                  type="time"
                  className={styles.timeInput}
                  value={draft.time}
                  onChange={handleTimeChange}
                  required
                />
                {scheduleErrors.time && (
                  <span className={styles.timeError}>{scheduleErrors.time}</span>
                )}
              </div>
            </div>
            <div className={styles.frequency} role="radiogroup" aria-label="Периодичность">
              <span className={styles.frequencyLabel}>Периодичность</span>
              <div className={styles.chips}>
                {PERIODICITY_OPTIONS.map((option) => {
                  const isSelected = draft.periodicity === option.value;
                  return (
                    <button
                      key={option.value}
                      type="button"
                      role="radio"
                      aria-checked={isSelected}
                      className={clsx(styles.chip, isSelected && styles.chipSelected)}
                      onClick={() =>
                        setDraft((prev) => ({ ...prev, periodicity: option.value }))
                      }
                    >
                      {option.label}
                    </button>
                  );
                })}
              </div>
            </div>
            <p className={styles.emailHint}>
              <Bell aria-hidden="true" />
              Напоминание придёт на email
            </p>
          </section>
        )}
      </div>

      <div className={styles.footer}>
        <Button
          variant="primary"
          size="large"
          fullWidth
          loading={isSubmitting}
          disabled={isNextDisabled}
          onClick={step === 1 ? handleNext : handleSubmit}
        >
          {step === 1
            ? 'Далее'
            : mode === 'edit'
              ? 'Сохранить'
              : 'Создать напоминание'}
        </Button>
      </div>
    </div>
  );
}
