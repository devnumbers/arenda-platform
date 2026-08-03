// PROTOTYPE — throwaway, issue #105
'use client';

import { type JSX, useState } from 'react';
import { PageHeader } from '@/shared/ui/page-header';
import { Button } from '@/shared/ui/button';
import { ConfirmModal } from '@/shared/ui/confirm-modal';
import { Home } from '@/shared/assets/icons';
import { prototypeHref, PROTOTYPE_ROUTES, type PrototypeVariant } from '../../model/variant';
import {
  formatPrototypeDate,
  prototypeDraftFromReminder,
  prototypeFrequencyLabels,
  prototypeItemReminder,
  prototypePropertyName,
} from '../../model/mocks';
import { VariantBCreate } from './VariantBCreate';
import styles from './VariantBItem.module.css';

export const variantName = 'Визард';

export type VariantBItemProps = {
  readonly variant: PrototypeVariant;
};

export function VariantBItem({ variant }: VariantBItemProps): JSX.Element {
  const reminder = prototypeItemReminder;

  const [isEditing, setIsEditing] = useState(false);
  const [isDeleteOpen, setIsDeleteOpen] = useState(false);

  const handleConfirmDelete = () => {
    console.log('[prototype#105] variant B — удалить напоминание', reminder.id);
  };

  if (isEditing) {
    return (
      <VariantBCreate
        variant={variant}
        mode="edit"
        initialDraft={prototypeDraftFromReminder(reminder)}
        onDone={() => setIsEditing(false)}
      />
    );
  }

  return (
    <div className={styles.root}>
      <PageHeader
        title="Напоминание"
        backHref={prototypeHref(PROTOTYPE_ROUTES.objectBlock, variant)}
      />

      <section className={styles.hero}>
        <p className={styles.heroCountdown}>Через 5 дней</p>
        <p className={styles.heroWhen}>
          {formatPrototypeDate(reminder.date)} в {reminder.time}
        </p>
        <h2 className={styles.heroTitle}>{reminder.title}</h2>
        <div className={styles.heroMeta}>
          <span className={styles.metaChip}>
            <Home aria-hidden="true" />
            {prototypePropertyName(reminder.propertyId)}
          </span>
          <span className={styles.metaChip}>
            {prototypeFrequencyLabels[reminder.frequency]}
          </span>
        </div>
      </section>

      <div className={styles.actions}>
        <Button variant="primary" size="large" onClick={() => setIsEditing(true)}>
          Изменить
        </Button>
        <Button variant="secondary" size="large" onClick={() => setIsDeleteOpen(true)}>
          Удалить
        </Button>
      </div>

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
