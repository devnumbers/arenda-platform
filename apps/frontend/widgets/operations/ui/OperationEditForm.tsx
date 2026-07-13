'use client';

import {
  type ChangeEvent,
  type FormEvent,
  useMemo,
  useState,
  type JSX,
} from 'react';
import { useParams, useRouter } from 'next/navigation';
import { PageHeader } from '@/shared/ui/page-header';
import { Button } from '@/shared/ui/button';
import { TextField } from '@/shared/ui/text-field';
import { DateSelect } from '@/shared/ui/date-select';
import { IconButton } from '@/shared/ui/icon-button';
import { Cancel } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { ApiError } from '@/shared/api/errors';
import {
  type OperationCategory,
  type OperationType,
} from '@/entities/operation/model/types';
import { getCategoriesByType } from '@/entities/operation/lib/categories';
import {
  useOperation,
  useUpdateOperation,
} from '@/features/operations/api/hooks';
import { FinanceLoading } from '@/widgets/finance/ui/FinanceLoading';
import { FinanceErrorState } from '@/widgets/finance/ui/FinanceErrorState';
import { SubscriptionReadonlyBanner } from '@/widgets/finance/ui/SubscriptionReadonlyBanner';
import { useSubscription } from '@/features/subscription/api/hooks';
import { isSubscriptionReadonly } from '@/features/subscription/lib/is-subscription-readonly';
import { CategorySelect } from './CategorySelect';
import { TypeSelect } from './TypeSelect';
import { ReminderSection } from './ReminderSection';
import styles from './OperationEditForm.module.css';

type FormData = {
  name: string;
  type: OperationType;
  category: OperationCategory | undefined;
  amount: string;
  operation_date: string;
  comment: string;
  reminderEnabled: boolean;
  reminderOffsetDays: 1 | 3 | 7;
};

type FormErrors = {
  name?: string;
  amount?: string;
  category?: string;
  operation_date?: string;
};

type Operation = {
  readonly id: string;
  readonly property_id: string;
  readonly name: string;
  readonly type: OperationType;
  readonly category: OperationCategory;
  readonly amount_kopecks: number;
  readonly operation_date: string;
  readonly comment?: string | null;
  readonly reminder_offset_days?: 1 | 3 | 7 | null;
};

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

function formatErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    return error.detail;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return 'Не удалось сохранить изменения. Попробуйте ещё раз.';
}

function useOperationId(): string | undefined {
  const params = useParams<{ readonly id: string }>();
  return params?.id;
}

function OperationEditFormContent({
  id,
  operation,
  readonly,
}: {
  readonly id: string;
  readonly operation: Operation;
  readonly readonly: boolean;
}): JSX.Element {
  const router = useRouter();
  const updateOperation = useUpdateOperation();

  const [form, setForm] = useState<FormData>({
    name: operation.name,
    type: operation.type,
    category: operation.category,
    amount: formatAmountFromKopecks(operation.amount_kopecks),
    operation_date: operation.operation_date,
    comment: operation.comment ?? '',
    reminderEnabled: operation.reminder_offset_days !== null && operation.reminder_offset_days !== undefined,
    reminderOffsetDays: operation.reminder_offset_days ?? 1,
  });
  const [errors, setErrors] = useState<FormErrors>({});

  const hasChanges = useMemo(() => {
    const amountKopecks = parseAmountToKopecks(form.amount);
    const currentReminder = form.reminderEnabled ? form.reminderOffsetDays : null;
    const originalReminder = operation.reminder_offset_days ?? null;

    return (
      form.name.trim() !== operation.name ||
      form.type !== operation.type ||
      form.category !== operation.category ||
      amountKopecks !== operation.amount_kopecks ||
      form.operation_date !== operation.operation_date ||
      (form.comment.trim() || undefined) !== (operation.comment ?? undefined) ||
      currentReminder !== originalReminder
    );
  }, [
    form.name,
    form.type,
    form.category,
    form.amount,
    form.operation_date,
    form.comment,
    form.reminderEnabled,
    form.reminderOffsetDays,
    operation,
  ]);

  const handleTypeChange = (type: OperationType) => {
    setForm((prev) => {
      const validCategories = getCategoriesByType(type);
      const category = validCategories.some((option) => option.value === prev.category)
        ? prev.category
        : undefined;
      return { ...prev, type, category };
    });
  };

  const handleNameChange = (event: ChangeEvent<HTMLInputElement>) => {
    const value = event.currentTarget.value;
    setForm((prev) => ({ ...prev, name: value }));
  };

  const handleAmountChange = (event: ChangeEvent<HTMLInputElement>) => {
    const value = event.currentTarget.value;
    setForm((prev) => ({ ...prev, amount: value }));
  };

  const handleDateChange = (value: string | undefined) => {
    setForm((prev) => ({ ...prev, operation_date: value ?? '' }));
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

  const validate = (): boolean => {
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

    if (!form.operation_date) {
      next.operation_date = 'Выберите дату';
    }

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

    updateOperation.mutate(
      {
        id,
        propertyId: operation.property_id,
        data: {
          name: form.name.trim(),
          type: form.type,
          category: form.category,
          amount_kopecks: amountKopecks,
          operation_date: form.operation_date,
          comment: form.comment.trim() || undefined,
          reminder_offset_days: form.reminderEnabled ? form.reminderOffsetDays : 0,
        },
      },
      {
        onSuccess: () => {
          router.push(ROUTES.financeOperation(id));
        },
      },
    );
  };

  return (
    <form className={styles.form} onSubmit={handleSubmit}>
      <div className={styles.fields}>
        <TypeSelect value={form.type} onChange={handleTypeChange} />
        <CategorySelect
          type={form.type}
          value={form.category}
          onChange={(category) => setForm((prev) => ({ ...prev, category }))}
          error={errors.category}
        />
        <TextField
          label="Название операции"
          placeholder="Например, аренда за июнь"
          required
          maxLength={50}
          showCounter
          fullWidth
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
          value={form.amount}
          onChange={handleAmountChange}
          error={errors.amount}
        />
        <DateSelect
          label="Дата операции"
          value={form.operation_date}
          onChange={handleDateChange}
          required
          error={errors.operation_date}
        />
        <TextField
          label="Комментарий"
          placeholder="Дополнительная информация"
          multiline
          maxLength={500}
          showCounter
          fullWidth
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

      {updateOperation.error && (
        <p className={styles.error} role="alert">
          {formatErrorMessage(updateOperation.error)}
        </p>
      )}

      <div className={styles.actions}>
        <Button
          type="submit"
          variant="primary"
          size="large"
          fullWidth
          loading={updateOperation.isPending}
          disabled={readonly || updateOperation.isPending || !hasChanges}
        >
          Сохранить
        </Button>
      </div>
    </form>
  );
}

export function OperationEditForm(): JSX.Element {
  const id = useOperationId();
  const router = useRouter();
  const { data, isLoading, isError, refetch, isFetching } = useOperation(id ?? '');
  const { data: subscription, isPending: isSubscriptionPending } = useSubscription();
  const readonly = isSubscriptionPending || isSubscriptionReadonly(subscription);

  const headerActions = id ? (
    <IconButton
      variant="secondary"
      size="large"
      icon={<Cancel />}
      aria-label="Отменить"
      onClick={() => router.push(ROUTES.financeOperation(id))}
    />
  ) : undefined;

  return (
    <div className={styles.root}>
      <PageHeader
        title="Редактирование операции"
        backHref={ROUTES.financeOperations}
        actions={headerActions}
      />

      <SubscriptionReadonlyBanner />

      {!id && (
        <FinanceErrorState
          onRetry={() => router.push(ROUTES.financeOperations)}
          isLoading={false}
        />
      )}

      {id && isLoading && <FinanceLoading />}

      {id && !isLoading && isError && (
        <FinanceErrorState onRetry={refetch} isLoading={isFetching} />
      )}

      {id && !isLoading && !isError && data && (
        <OperationEditFormContent
          id={id}
          operation={{...data, category: data.category as OperationCategory}}
          readonly={readonly}
        />
      )}
    </div>
  );
}
