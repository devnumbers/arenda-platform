// PROTOTYPE — throwaway, issue #105
'use client';

import { type JSX, useState } from 'react';
import { PageHeader } from '@/shared/ui/page-header';
import { Button } from '@/shared/ui/button';
import { ConfirmModal } from '@/shared/ui/confirm-modal';
import { prototypeHref, PROTOTYPE_ROUTES, type PrototypeVariant } from '../../model/variant';
import {
  formatPrototypeDateTime,
  prototypeDraftFromReminder,
  prototypeFrequencyLabels,
  prototypeItemReminder,
  prototypePropertyName,
  type PrototypeReminderDraft,
} from '../../model/mocks';
import { VariantAReminderForm } from './VariantAReminderForm';
import styles from './VariantAItem.module.css';

export const variantName = 'Одноэкранная форма';

export type VariantAItemProps = {
  readonly variant: PrototypeVariant;
};

export function VariantAItem({ variant }: VariantAItemProps): JSX.Element {
  const reminder = prototypeItemReminder;

  const [isEditing, setIsEditing] = useState(false);
  const [isDeleteOpen, setIsDeleteOpen] = useState(false);

  const handleSave = (draft: PrototypeReminderDraft) => {
    console.log('[prototype#105] variant A — сохранить напоминание', draft);
    setIsEditing(false);
  };

  const handleConfirmDelete = () => {
    console.log('[prototype#105] variant A — удалить напоминание', reminder.id);
  };

  return (
    <div className={styles.root}>
      <PageHeader
        title="Напоминание"
        backHref={prototypeHref(PROTOTYPE_ROUTES.objectBlock, variant)}
      />

      {isEditing ? (
        <VariantAReminderForm
          initialDraft={prototypeDraftFromReminder(reminder)}
          submitLabel="Сохранить"
          onSubmit={handleSave}
        />
      ) : (
        <>
          <section className={styles.card}>
            <h2 className={styles.name}>{reminder.title}</h2>
            <dl className={styles.details}>
              <div className={styles.detailRow}>
                <dt className={styles.detailLabel}>Объект</dt>
                <dd className={styles.detailValue}>{prototypePropertyName(reminder.propertyId)}</dd>
              </div>
              <div className={styles.detailRow}>
                <dt className={styles.detailLabel}>Дата и время</dt>
                <dd className={styles.detailValue}>{formatPrototypeDateTime(reminder)}</dd>
              </div>
              <div className={styles.detailRow}>
                <dt className={styles.detailLabel}>Периодичность</dt>
                <dd className={styles.detailValue}>
                  {prototypeFrequencyLabels[reminder.frequency]}
                </dd>
              </div>
              <div className={styles.detailRow}>
                <dt className={styles.detailLabel}>Канал</dt>
                <dd className={styles.detailValue}>Email</dd>
              </div>
            </dl>
          </section>

          <div className={styles.actions}>
            <Button variant="primary" size="large" onClick={() => setIsEditing(true)}>
              Редактировать
            </Button>
            <Button variant="secondary" size="large" onClick={() => setIsDeleteOpen(true)}>
              Удалить
            </Button>
          </div>
        </>
      )}

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
