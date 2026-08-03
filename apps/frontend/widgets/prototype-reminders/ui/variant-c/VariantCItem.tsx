// PROTOTYPE — throwaway, issue #105
'use client';

import { type ChangeEvent, type FormEvent, type JSX, useId, useMemo, useState } from 'react';
import clsx from 'clsx';
import { PageHeader } from '@/shared/ui/page-header';
import { Button } from '@/shared/ui/button';
import { IconButton } from '@/shared/ui/icon-button';
import { TextField } from '@/shared/ui/text-field';
import { DateSelect } from '@/shared/ui/date-select';
import { Select } from '@/shared/ui/select';
import { ConfirmModal } from '@/shared/ui/confirm-modal';
import { Bell, Trash } from '@/shared/assets/icons';
import { prototypeHref, PROTOTYPE_ROUTES, type PrototypeVariant } from '../../model/variant';
import {
  prototypeDraftFromReminder,
  prototypeFrequencyOptions,
  prototypeItemReminder,
  prototypeProperties,
  type PrototypeReminderDraft,
} from '../../model/mocks';
import styles from './VariantCItem.module.css';

export const variantName = 'Форма с живой сводкой';

export type VariantCItemProps = {
  readonly variant: PrototypeVariant;
};

const objectOptions = prototypeProperties.map((property) => ({
  value: property.id,
  label: property.name,
}));

export function VariantCItem({ variant }: VariantCItemProps): JSX.Element {
  const reminder = prototypeItemReminder;
  const initialDraft = useMemo(() => prototypeDraftFromReminder(reminder), [reminder]);

  const timeInputId = useId();
  const frequencyLabelId = useId();

  const [draft, setDraft] = useState<PrototypeReminderDraft>(initialDraft);
  const [isDeleteOpen, setIsDeleteOpen] = useState(false);

  const hasChanges = useMemo(
    () =>
      draft.title !== initialDraft.title ||
      draft.date !== initialDraft.date ||
      draft.time !== initialDraft.time ||
      draft.frequency !== initialDraft.frequency ||
      draft.propertyId !== initialDraft.propertyId,
    [draft, initialDraft],
  );

  const handleTitleChange = (event: ChangeEvent<HTMLInputElement>) => {
    setDraft((prev) => ({ ...prev, title: event.currentTarget.value }));
  };

  const handleTimeChange = (event: ChangeEvent<HTMLInputElement>) => {
    setDraft((prev) => ({ ...prev, time: event.currentTarget.value }));
  };

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    console.log('[prototype#105] variant C — сохранить напоминание', draft);
  };

  const handleConfirmDelete = () => {
    console.log('[prototype#105] variant C — удалить напоминание', reminder.id);
  };

  const headerActions = (
    <IconButton
      variant="secondary"
      size="large"
      icon={<Trash />}
      aria-label="Удалить напоминание"
      onClick={() => setIsDeleteOpen(true)}
    />
  );

  return (
    <div className={styles.root}>
      <PageHeader
        title="Напоминание"
        backHref={prototypeHref(PROTOTYPE_ROUTES.objectBlock, variant)}
        actions={headerActions}
      />

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
          <p className={styles.emailHint}>
            <Bell aria-hidden="true" />
            Напоминание придёт на email
          </p>
        </div>

        <Button type="submit" variant="primary" size="large" fullWidth disabled={!hasChanges}>
          Сохранить
        </Button>
      </form>

      <ConfirmModal
        isOpen={isDeleteOpen}
        title="Удалить напоминание?"
        description="Напоминание будет удалено безвозвратно."
        confirmLabel="Удалить"
        onClose={() => setIsDeleteOpen(false)}
        onConfirm={handleConfirmDelete}
      />
    </div>
  );
}
