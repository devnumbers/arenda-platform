'use client';

import { useState, type JSX } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { PageHeader } from '@/shared/ui/page-header';
import { Button } from '@/shared/ui/button';
import { ConfirmModal } from '@/shared/ui/confirm-modal';
import { Home } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { notify } from '@/shared/lib/notifications';
import {
  formatCountdownFromNow,
  formatReminderDateTime,
} from '@/shared/lib/datetime';
import { useFreeReminder, useDeleteFreeReminder } from '@/features/free-reminders';
import { PERIODICITY_LABELS } from '@/features/free-reminders';
import { useProperty } from '@/features/properties';
import { FreeReminderWizard } from './FreeReminderWizard';
import styles from './FreeReminderDetailPage.module.css';

export function FreeReminderDetailPage(): JSX.Element {
  const params = useParams<{ readonly id: string }>();
  const router = useRouter();
  const id = params?.id;

  const { data: reminder, isLoading, isError } = useFreeReminder(id ?? '');
  const { data: property } = useProperty(reminder?.property_id ?? '');

  const deleteMutation = useDeleteFreeReminder();
  const [isEditing, setIsEditing] = useState(false);
  const [isDeleteOpen, setIsDeleteOpen] = useState(false);

  const handleConfirmDelete = (): void => {
    if (!reminder) return;
    deleteMutation.mutate(reminder.id, {
      onSuccess: () => {
        setIsDeleteOpen(false);
        notify.scenarios.freeReminders.deleted();
        goBack(router, ROUTES.properties);
      },
      onError: (error) => {
        notify.scenarios.freeReminders.deleteError(error);
      },
    });
  };

  if (isEditing && reminder) {
    return (
      <FreeReminderWizard
        mode="edit"
        reminder={reminder}
        onDone={() => setIsEditing(false)}
      />
    );
  }

  return (
    <div className={styles.root}>
      <PageHeader title="Напоминание" backHref={ROUTES.properties} />

      {isLoading && <p>Загрузка…</p>}

      {isError && <p>Не удалось загрузить напоминание.</p>}

      {!isLoading && !isError && reminder && (
        <>
          <section className={styles.hero}>
            <p className={styles.heroCountdown}>
              {formatCountdownFromNow(reminder.trigger_at)}
            </p>
            <p className={styles.heroWhen}>
              {(() => {
                const { date, time } = formatReminderDateTime(reminder.trigger_at);
                return `${date} в ${time}`;
              })()}
            </p>
            <h2 className={styles.heroTitle}>{reminder.title}</h2>
            <div className={styles.heroMeta}>
              <span className={styles.metaChip}>
                <Home aria-hidden="true" />
                {property?.name ?? 'Без объекта'}
              </span>
              <span className={styles.metaChip}>
                {PERIODICITY_LABELS[reminder.periodicity]}
              </span>
            </div>
          </section>

          <div className={styles.actions}>
            <Button variant="primary" size="large" onClick={() => setIsEditing(true)}>
              Изменить
            </Button>
            <Button
              variant="secondary"
              size="large"
              onClick={() => setIsDeleteOpen(true)}
            >
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
