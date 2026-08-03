// PROTOTYPE — throwaway, issue #105
'use client';

import { type ChangeEvent, type FormEvent, type JSX, useId, useState } from 'react';
import clsx from 'clsx';
import { Button } from '@/shared/ui/button';
import { TextField } from '@/shared/ui/text-field';
import { DateSelect } from '@/shared/ui/date-select';
import { Select } from '@/shared/ui/select';
import { Bell } from '@/shared/assets/icons';
import {
  prototypeEmptyDraft,
  prototypeFrequencyOptions,
  prototypeProperties,
  type PrototypeReminderDraft,
} from '../../model/mocks';
import styles from './VariantAReminderForm.module.css';

export type VariantAReminderFormProps = {
  readonly initialDraft?: PrototypeReminderDraft;
  readonly preselectedObjectId?: string;
  readonly submitLabel: string;
  readonly onSubmit: (draft: PrototypeReminderDraft) => void;
};

const objectOptions = prototypeProperties.map((property) => ({
  value: property.id,
  label: property.name,
}));

export function VariantAReminderForm({
  initialDraft,
  preselectedObjectId,
  submitLabel,
  onSubmit,
}: VariantAReminderFormProps): JSX.Element {
  const timeInputId = useId();
  const frequencyLabelId = useId();

  const [draft, setDraft] = useState<PrototypeReminderDraft>(() => ({
    ...(initialDraft ?? prototypeEmptyDraft),
    propertyId: initialDraft?.propertyId ?? preselectedObjectId,
  }));

  const handleTitleChange = (event: ChangeEvent<HTMLInputElement>) => {
    setDraft((prev) => ({ ...prev, title: event.currentTarget.value }));
  };

  const handleDateChange = (value: string | undefined) => {
    setDraft((prev) => ({ ...prev, date: value ?? '' }));
  };

  const handleTimeChange = (event: ChangeEvent<HTMLInputElement>) => {
    setDraft((prev) => ({ ...prev, time: event.currentTarget.value }));
  };

  const handleObjectChange = (value: string) => {
    setDraft((prev) => ({ ...prev, propertyId: value }));
  };

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    onSubmit(draft);
  };

  return (
    <form className={styles.form} onSubmit={handleSubmit}>
      <div className={styles.fields}>
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
        <div className={styles.datetimeRow}>
          <DateSelect
            label="Дата"
            value={draft.date || undefined}
            onChange={handleDateChange}
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
        <div className={styles.frequency} role="radiogroup" aria-labelledby={frequencyLabelId}>
          <span id={frequencyLabelId} className={styles.frequencyLabel}>
            Периодичность
          </span>
          <div className={styles.frequencyOptions}>
            {prototypeFrequencyOptions.map((option) => {
              const isSelected = draft.frequency === option.value;
              return (
                <button
                  key={option.value}
                  type="button"
                  role="radio"
                  aria-checked={isSelected}
                  className={clsx(styles.frequencyOption, isSelected && styles.frequencySelected)}
                  onClick={() => setDraft((prev) => ({ ...prev, frequency: option.value }))}
                >
                  <span className={styles.radio} aria-hidden="true">
                    <span className={styles.radioDot} />
                  </span>
                  {option.label}
                </button>
              );
            })}
          </div>
        </div>
        <Select
          label="Объект"
          placeholder="Выберите объект"
          required
          options={objectOptions}
          value={draft.propertyId}
          onChange={handleObjectChange}
        />
        <p className={styles.emailHint}>
          <Bell aria-hidden="true" />
          Напоминание придёт на email
        </p>
      </div>

      <Button type="submit" variant="primary" size="large" fullWidth>
        {submitLabel}
      </Button>
    </form>
  );
}
