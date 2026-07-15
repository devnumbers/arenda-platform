'use client';

import {
  type ChangeEvent,
  type FormEvent,
  useId,
  useMemo,
  useState,
  type JSX,
} from 'react';
import { useParams, useRouter } from 'next/navigation';
import clsx from 'clsx';
import { Modal } from '@heroui/react';
import { notify } from '@/shared/lib/notifications';
import { PageHeader } from '@/shared/ui/page-header';
import { Button } from '@/shared/ui/button';
import { TextField } from '@/shared/ui/text-field';
import { DatePickerField } from '@/shared/ui/date-picker-field';
import { ROUTES } from '@/shared/config/routes';
import type { components } from '@/shared/api/generated';
import { type OperationType } from '@/entities/operation/model/types';
import {
  useRecurringOperation,
  useUpdateRecurringOperation,
  useDeleteRecurringOperation,
} from '@/features/recurring-operations/api/hooks';
import { FinanceLoading } from '@/widgets/finance/ui/FinanceLoading';
import { FinanceErrorState } from '@/widgets/finance/ui/FinanceErrorState';
import { SubscriptionReadonlyBanner } from '@/widgets/finance/ui/SubscriptionReadonlyBanner';
import { useSubscription } from '@/features/subscription/api/hooks';
import { isSubscriptionReadonly } from '@/features/subscription/lib/is-subscription-readonly';
import { CategorySelect } from './CategorySelect';
import { TypeSelect } from './TypeSelect';
import { ReminderSection } from './ReminderSection';
import frequencyStyles from './FrequencySelect.module.css';
import styles from './RecurringOperationEditPage.module.css';

type RecurringOperationResponse =
  components['schemas']['RecurringOperationResponse'];
type RecurringOperationUpdateRequest =
  components['schemas']['RecurringOperationUpdateRequest'];

type RecurringPeriodicity = 'monthly' | 'yearly';

type FormData = {
  type: OperationType;
  category: string | undefined;
  name: string;
  amount: string;
  periodicity: RecurringPeriodicity;
  endDate: string;
  comment: string;
  reminderEnabled: boolean;
  reminderOffsetDays: 1 | 3 | 7;
};

type FormErrors = {
  name?: string;
  amount?: string;
  category?: string;
  endDate?: string;
};

const PERIODICITY_OPTIONS: {
  readonly value: RecurringPeriodicity;
  readonly label: string;
}[] = [
  { value: 'monthly', label: 'Ежемесячно' },
  { value: 'yearly', label: 'Ежегодно' },
];

type SeriesFrequencySelectProps = {
  readonly value: RecurringPeriodicity;
  readonly onChange: (value: RecurringPeriodicity) => void;
  readonly error?: string;
  readonly disabled?: boolean;
};

function SeriesFrequencySelect({
  value,
  onChange,
  error,
  disabled,
}: SeriesFrequencySelectProps): JSX.Element {
  const labelId = useId();

  return (
    <div
      className={clsx(frequencyStyles.root, error && frequencyStyles.error)}
      role="radiogroup"
      aria-labelledby={labelId}
    >
      <span id={labelId} className={frequencyStyles.label}>
        Периодичность
      </span>
      <div className={frequencyStyles.options}>
        {PERIODICITY_OPTIONS.map((option) => {
          const isSelected = value === option.value;
          return (
            <button
              key={option.value}
              type="button"
              role="radio"
              aria-checked={isSelected}
              disabled={disabled}
              className={clsx(
                frequencyStyles.option,
                isSelected && frequencyStyles.selected,
              )}
              onClick={() => onChange(option.value)}
            >
              <span className={frequencyStyles.radio} aria-hidden="true">
                <span className={frequencyStyles.radioDot} />
              </span>
              {option.label}
            </button>
          );
        })}
      </div>
      {error && <span className={frequencyStyles.errorText}>{error}</span>}
    </div>
  );
}

function formatAmountFromKopecks(kopecks: number): string {
  return (kopecks / 100).toFixed(2);
}

function parseAmountToKopecks(amount: string): number | undefined {
  const normalized = amount.trim().replace(',', '.');
  if (normalized === '') {
    return undefined;
  }
  const value = Number(normalized);
  if (Number.isNaN(value) || value <= 0) {
    return undefined;
  }
  return Math.round(value * 100);
}

function validateForm(form: FormData): FormErrors {
  const next: FormErrors = {};

  if (form.name.trim() === '') {
    next.name = 'Введите название операции';
  }

  if (parseAmountToKopecks(form.amount) === undefined) {
    next.amount = 'Введите сумму больше 0';
  }

  if (!form.category) {
    next.category = 'Выберите категорию';
  }

  if (form.endDate) {
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    if (new Date(form.endDate) < today) {
      next.endDate = 'Дата окончания не может быть в прошлом';
    }
  }

  return next;
}

function useRecurringOperationId(): string | undefined {
  const params = useParams<{ readonly id: string }>();
  return params?.id;
}

type DeleteSeriesModalProps = {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onConfirm: () => void;
  readonly isLoading: boolean;
};

function DeleteSeriesModal({
  isOpen,
  onClose,
  onConfirm,
  isLoading,
}: DeleteSeriesModalProps): JSX.Element {
  const handleOpenChange = (open: boolean): void => {
    if (!open) {
      onClose();
    }
  };

  return (
    <Modal>
      <Modal.Backdrop isOpen={isOpen} onOpenChange={handleOpenChange}>
        <Modal.Container placement="center" size="sm">
          <Modal.Dialog aria-label="Удалить серию">
            <Modal.Header>
              <Modal.Heading>Удалить серию?</Modal.Heading>
            </Modal.Header>
            <Modal.Body>
              <p>
                Будущие операции будут удалены, прошедшие останутся.
              </p>
            </Modal.Body>
            <Modal.Footer>
              <Button variant="secondary" onClick={onClose} type="button">
                Отменить
              </Button>
              <Button
                variant="primary"
                onClick={onConfirm}
                type="button"
                loading={isLoading}
              >
                Удалить
              </Button>
            </Modal.Footer>
          </Modal.Dialog>
        </Modal.Container>
      </Modal.Backdrop>
    </Modal>
  );
}

function RecurringOperationEditPageContent({
  id,
  operation,
  readonly,
}: {
  readonly id: string;
  readonly operation: RecurringOperationResponse;
  readonly readonly: boolean;
}): JSX.Element {
  const router = useRouter();
  const updateOperation = useUpdateRecurringOperation();
  const deleteOperation = useDeleteRecurringOperation();

  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
  const canDelete = !readonly && !operation.lease_id;

  const [form, setForm] = useState<FormData>({
    type: operation.type,
    category: operation.category_id,
    name: operation.name,
    amount: formatAmountFromKopecks(operation.amount_kopecks),
    periodicity: operation.periodicity,
    endDate: operation.end_date ?? '',
    comment: operation.comment ?? '',
    reminderEnabled:
      operation.reminder_offset_days !== null &&
      operation.reminder_offset_days !== undefined,
    reminderOffsetDays: operation.reminder_offset_days ?? 1,
  });
  const [errors, setErrors] = useState<FormErrors>({});

  const handleTypeChange = (type: OperationType) => {
    setForm((prev) => ({ ...prev, type, category: undefined }));
  };

  const handleNameChange = (event: ChangeEvent<HTMLInputElement>) => {
    const value = event.currentTarget.value;
    setForm((prev) => ({ ...prev, name: value }));
  };

  const handleAmountChange = (event: ChangeEvent<HTMLInputElement>) => {
    const value = event.currentTarget.value;
    setForm((prev) => ({ ...prev, amount: value }));
  };

  const handleEndDateChange = (value: string) => {
    setForm((prev) => ({ ...prev, endDate: value }));
  };

  const handleCommentChange = (event: ChangeEvent<HTMLTextAreaElement>) => {
    const value = event.currentTarget.value;
    setForm((prev) => ({ ...prev, comment: value }));
  };

  const handleReminderToggle = (enabled: boolean) => {
    setForm((prev) => ({ ...prev, reminderEnabled: enabled }));
  };

  const handleReminderOffsetChange = (offsetDays: 1 | 3 | 7) => {
    setForm((prev) => ({ ...prev, reminderOffsetDays: offsetDays }));
  };

  const isFormValid = useMemo(
    () => Object.keys(validateForm(form)).length === 0,
    [form],
  );

  const hasChanges = useMemo(() => {
    const amountKopecks = parseAmountToKopecks(form.amount);
    const currentReminder = form.reminderEnabled ? form.reminderOffsetDays : null;
    const originalReminder = operation.reminder_offset_days ?? null;

    return (
      form.name.trim() !== operation.name ||
      form.type !== operation.type ||
      form.category !== operation.category_id ||
      amountKopecks !== operation.amount_kopecks ||
      form.periodicity !== operation.periodicity ||
      (form.endDate || undefined) !== (operation.end_date ?? undefined) ||
      (form.comment.trim() || undefined) !== (operation.comment ?? undefined) ||
      currentReminder !== originalReminder
    );
  }, [
    form.name,
    form.type,
    form.category,
    form.amount,
    form.periodicity,
    form.endDate,
    form.comment,
    form.reminderEnabled,
    form.reminderOffsetDays,
    operation,
  ]);

  const validate = (): boolean => {
    const next = validateForm(form);
    setErrors(next);
    return Object.keys(next).length === 0;
  };

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (!validate()) {
      return;
    }

    const amountKopecks = parseAmountToKopecks(form.amount);
    if (amountKopecks === undefined || !form.category) {
      return;
    }

    const data: RecurringOperationUpdateRequest = {
      type: form.type,
      category_id: form.category,
      name: form.name.trim(),
      amount_kopecks: amountKopecks,
      periodicity: form.periodicity,
      reminder_offset_days: form.reminderEnabled ? form.reminderOffsetDays : 0,
    };

    if (form.endDate) {
      data.end_date = form.endDate;
    }

    if (form.comment.trim()) {
      data.comment = form.comment.trim();
    }

    updateOperation.mutate(
      {
        id,
        propertyId: operation.property_id,
        data,
      },
      {
        onSuccess: () => {
          router.push(ROUTES.financeOperations);
        },
        onError: (error) => {
          notify.scenarios.operations.recurringOperationSaveError(error);
        },
      },
    );
  };

  const handleCancel = () => {
    router.push(ROUTES.financeOperations);
  };

  const handleDeleteClick = () => {
    setIsDeleteModalOpen(true);
  };

  const handleCloseDeleteModal = () => {
    setIsDeleteModalOpen(false);
  };

  const handleConfirmDelete = () => {
    const promise = deleteOperation.mutateAsync({
      id,
      propertyId: operation.property_id,
    });

    void notify.scenarios.operations.recurringOperationDeleted(promise);

    promise
      .then(() => {
        setIsDeleteModalOpen(false);
        router.push(ROUTES.financeOperations);
      })
      .catch(() => {}); // error is already reported by notify.promise
  };

  return (
    <form className={styles.form} onSubmit={handleSubmit}>
      <div className={styles.fields}>
        <TypeSelect
          value={form.type}
          onChange={handleTypeChange}
          disabled={readonly}
        />
        <CategorySelect
          type={form.type}
          value={form.category}
          onChange={(category) => setForm((prev) => ({ ...prev, category }))}
          error={errors.category}
          disabled={readonly}
        />
        <TextField
          label="Название операции"
          placeholder="Например, аренда за июнь"
          required
          maxLength={50}
          showCounter
          fullWidth
          disabled={readonly}
          value={form.name}
          onChange={handleNameChange}
          error={errors.name}
        />
        <TextField
          label="Сумма, ₽"
          placeholder="0"
          type="number"
          min={0}
          step="0.01"
          required
          fullWidth
          disabled={readonly}
          value={form.amount}
          onChange={handleAmountChange}
          error={errors.amount}
        />
        <SeriesFrequencySelect
          value={form.periodicity}
          onChange={(periodicity) => setForm((prev) => ({ ...prev, periodicity }))}
          disabled={readonly}
        />
        <DatePickerField
          label="Дата окончания (необязательно)"
          value={form.endDate}
          onChange={handleEndDateChange}
          disabled={readonly}
          fullWidth
          error={errors.endDate}
        />
        <TextField
          label="Комментарий"
          placeholder="Дополнительная информация"
          multiline
          maxLength={500}
          showCounter
          fullWidth
          disabled={readonly}
          value={form.comment}
          onChange={handleCommentChange}
        />
        <ReminderSection
          enabled={form.reminderEnabled}
          offsetDays={form.reminderOffsetDays}
          onEnabledChange={handleReminderToggle}
          onOffsetChange={handleReminderOffsetChange}
          disabled={readonly}
        />
      </div>

      <div className={styles.actions}>
        <Button
          type="submit"
          variant="primary"
          size="large"
          fullWidth
          loading={updateOperation.isPending}
          disabled={readonly || updateOperation.isPending || !isFormValid || !hasChanges}
        >
          Сохранить
        </Button>
        <Button
          type="button"
          variant="secondary"
          size="large"
          fullWidth
          onClick={handleCancel}
        >
          Отмена
        </Button>
      </div>

      {canDelete && (
        <div className={styles.deleteSection}>
          <Button
            type="button"
            variant="secondary"
            size="large"
            fullWidth
            className={styles.deleteButton}
            loading={deleteOperation.isPending}
            disabled={deleteOperation.isPending}
            onClick={handleDeleteClick}
          >
            Удалить серию
          </Button>
        </div>
      )}

      <DeleteSeriesModal
        isOpen={isDeleteModalOpen}
        onClose={handleCloseDeleteModal}
        onConfirm={handleConfirmDelete}
        isLoading={deleteOperation.isPending}
      />
    </form>
  );
}

export function RecurringOperationEditPage(): JSX.Element {
  const id = useRecurringOperationId();
  const router = useRouter();
  const {
    data,
    isLoading,
    isError,
    refetch,
    isFetching,
  } = useRecurringOperation(id ?? '');
  const { data: subscription, isPending: isSubscriptionPending } = useSubscription();
  const readonly = isSubscriptionPending || isSubscriptionReadonly(subscription);

  if (!id) {
    return (
      <div className={styles.root}>
        <FinanceErrorState
          onRetry={() => router.push(ROUTES.financeOperations)}
          isLoading={false}
        />
      </div>
    );
  }

  return (
    <div className={styles.root}>
      <PageHeader
        title="Редактирование серии"
        backHref={ROUTES.financeOperations}
      />

      <SubscriptionReadonlyBanner />

      {isLoading && <FinanceLoading />}

      {!isLoading && isError && (
        <FinanceErrorState onRetry={refetch} isLoading={isFetching} />
      )}

      {!isLoading && !isError && data && (
        <RecurringOperationEditPageContent
          id={id}
          operation={data}
          readonly={readonly}
        />
      )}
    </div>
  );
}
