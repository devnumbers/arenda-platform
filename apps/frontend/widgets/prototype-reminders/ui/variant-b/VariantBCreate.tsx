// PROTOTYPE — throwaway, issue #105
'use client';

import { type ChangeEvent, type JSX, useId, useState } from 'react';
import { useRouter } from 'next/navigation';
import clsx from 'clsx';
import { Button } from '@/shared/ui/button';
import { TextField } from '@/shared/ui/text-field';
import { DateSelect } from '@/shared/ui/date-select';
import { Select } from '@/shared/ui/select';
import { WizardHeader } from '@/shared/ui/wizard-header';
import { Bell } from '@/shared/assets/icons';
import { prototypeHref, PROTOTYPE_ROUTES, type PrototypeVariant } from '../../model/variant';
import {
  prototypeEmptyDraft,
  prototypeFrequencyOptions,
  prototypeProperties,
  type PrototypeReminderDraft,
} from '../../model/mocks';
import styles from './VariantBCreate.module.css';

export const variantName = 'Визард';

export type VariantBCreateProps = {
  readonly variant: PrototypeVariant;
  readonly preselectedObjectId?: string;
  readonly mode?: 'create' | 'edit';
  readonly initialDraft?: PrototypeReminderDraft;
  readonly onDone?: () => void;
};

const objectOptions = prototypeProperties.map((property) => ({
  value: property.id,
  label: property.name,
}));

export function VariantBCreate({
  variant,
  preselectedObjectId,
  mode = 'create',
  initialDraft,
  onDone,
}: VariantBCreateProps): JSX.Element {
  const router = useRouter();
  const timeInputId = useId();

  const [step, setStep] = useState<1 | 2>(1);
  const [draft, setDraft] = useState<PrototypeReminderDraft>(() => ({
    ...(initialDraft ?? prototypeEmptyDraft),
    propertyId: initialDraft?.propertyId ?? preselectedObjectId,
  }));

  const exitWizard = () => {
    if (mode === 'edit') {
      onDone?.();
      return;
    }
    router.push(prototypeHref(PROTOTYPE_ROUTES.objectBlock, variant));
  };

  const handleBack = () => {
    if (step === 2) {
      setStep(1);
      return;
    }
    exitWizard();
  };

  const handleFinish = () => {
    console.log(`[prototype#105] variant B — ${mode === 'edit' ? 'сохранить' : 'создать'} напоминание`, draft);
    onDone?.();
  };

  const handleTitleChange = (event: ChangeEvent<HTMLInputElement>) => {
    setDraft((prev) => ({ ...prev, title: event.currentTarget.value }));
  };

  const handleTimeChange = (event: ChangeEvent<HTMLInputElement>) => {
    setDraft((prev) => ({ ...prev, time: event.currentTarget.value }));
  };

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
            />
            <Select
              label="Объект"
              placeholder="Выберите объект"
              required
              options={objectOptions}
              value={draft.propertyId}
              onChange={(value) => setDraft((prev) => ({ ...prev, propertyId: value }))}
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
                onChange={(value) => setDraft((prev) => ({ ...prev, date: value ?? '' }))}
                required
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
              </div>
            </div>
            <div className={styles.frequency} role="radiogroup" aria-label="Периодичность">
              <span className={styles.frequencyLabel}>Периодичность</span>
              <div className={styles.chips}>
                {prototypeFrequencyOptions.map((option) => {
                  const isSelected = draft.frequency === option.value;
                  return (
                    <button
                      key={option.value}
                      type="button"
                      role="radio"
                      aria-checked={isSelected}
                      className={clsx(styles.chip, isSelected && styles.chipSelected)}
                      onClick={() => setDraft((prev) => ({ ...prev, frequency: option.value }))}
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
          onClick={step === 1 ? () => setStep(2) : handleFinish}
        >
          {step === 1 ? 'Далее' : mode === 'edit' ? 'Сохранить' : 'Создать напоминание'}
        </Button>
      </div>
    </div>
  );
}
