// PROTOTYPE — throwaway, issue #105
'use client';

import { type ChangeEvent, type FormEvent, type JSX, useId, useState } from 'react';
import clsx from 'clsx';
import { PageHeader } from '@/shared/ui/page-header';
import { Button } from '@/shared/ui/button';
import { TextField } from '@/shared/ui/text-field';
import { DateSelect } from '@/shared/ui/date-select';
import { Select } from '@/shared/ui/select';
import { Bell } from '@/shared/assets/icons';
import { prototypeHref, PROTOTYPE_ROUTES, type PrototypeVariant } from '../../model/variant';
import {
  prototypeEmptyDraft,
  prototypeFrequencyOptions,
  prototypeProperties,
  prototypePropertyName,
  type PrototypeReminderDraft,
} from '../../model/mocks';
import styles from './VariantCCreate.module.css';

export const variantName = 'Форма с живой сводкой';

export type VariantCCreateProps = {
  readonly variant: PrototypeVariant;
  readonly preselectedObjectId?: string;
};

const objectOptions = prototypeProperties.map((property) => ({
  value: property.id,
  label: property.name,
}));

const WEEKDAY_PHRASES = [
  'по воскресеньям',
  'по понедельникам',
  'по вторникам',
  'по средам',
  'по четвергам',
  'по пятницам',
  'по субботам',
];

function buildSummarySentence(draft: PrototypeReminderDraft): string {
  const date = draft.date || '2026-03-12';
  const time = draft.time || '10:00';
  const parsed = new Date(`${date}T${time}:00`);
  if (Number.isNaN(parsed.getTime())) {
    return 'Выберите дату и время';
  }

  const dateLong = parsed.toLocaleDateString('ru-RU', { day: 'numeric', month: 'long' });

  switch (draft.frequency) {
    case 'once':
      return `Разово, ${dateLong} в ${time}`;
    case 'daily':
      return `Каждый день в ${time}`;
    case 'weekly':
      return `Каждую неделю, ${WEEKDAY_PHRASES[parsed.getDay()]}, в ${time}`;
    case 'monthly':
      return `Каждый месяц, ${parsed.getDate()}-го числа, в ${time}`;
    case 'yearly':
      return `Каждый год, ${dateLong}, в ${time}`;
  }
}

export function VariantCCreate({
  variant,
  preselectedObjectId,
}: VariantCCreateProps): JSX.Element {
  const timeInputId = useId();
  const frequencyLabelId = useId();

  const [draft, setDraft] = useState<PrototypeReminderDraft>(() => ({
    ...prototypeEmptyDraft,
    propertyId: preselectedObjectId,
  }));

  const handleTitleChange = (event: ChangeEvent<HTMLInputElement>) => {
    setDraft((prev) => ({ ...prev, title: event.currentTarget.value }));
  };

  const handleTimeChange = (event: ChangeEvent<HTMLInputElement>) => {
    setDraft((prev) => ({ ...prev, time: event.currentTarget.value }));
  };

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    console.log('[prototype#105] variant C — создать напоминание', draft);
  };

  return (
    <div className={styles.root}>
      <PageHeader
        title="Новое напоминание"
        backHref={prototypeHref(PROTOTYPE_ROUTES.objectBlock, variant)}
      />

      <form className={styles.layout} onSubmit={handleSubmit}>
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
            onChange={(value) => setDraft((prev) => ({ ...prev, propertyId: value }))}
          />
        </div>

        <aside className={styles.summaryCard} aria-live="polite">
          <span className={styles.summaryLabel}>Как это будет работать</span>
          <p className={styles.summaryText}>
            {buildSummarySentence(draft)} — придёт письмо на email
          </p>
          <p className={styles.summaryObject}>
            {draft.propertyId
              ? `Объект: ${prototypePropertyName(draft.propertyId)}`
              : 'Объект не выбран'}
          </p>
          <p className={styles.summaryHint}>
            <Bell aria-hidden="true" />
            Сводка обновляется по мере заполнения
          </p>
        </aside>

        <div className={styles.submitRow}>
          <Button type="submit" variant="primary" size="large" fullWidth>
            Создать напоминание
          </Button>
        </div>
      </form>
    </div>
  );
}
