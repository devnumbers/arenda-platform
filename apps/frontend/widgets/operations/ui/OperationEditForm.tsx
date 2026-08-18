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
import { goBack } from '@/shared/lib/navigation';
import { notify } from '@/shared/lib/notifications';
import { type Operation, type OperationType } from '@/entities/operation';
import {
  useOperation,
  useUpdateOperation,
} from '@/features/operations';
import { FinanceLoading } from '@/shared/ui/finance-loading';
import { FinanceErrorState } from '@/shared/ui/finance-error-state';
import { SubscriptionReadonlyBanner } from '@/features/subscription';
import { useSubscription } from '@/features/subscription';
import { isSubscriptionReadonly } from '@/features/subscription';
import { CategorySelect } from './CategorySelect';
import { TypeSelect } from './TypeSelect';
import { ReminderSection } from './ReminderSection';
import styles from './OperationEditForm.module.css';

type FormData = {
  name: string;
  type: OperationType;
  categoryId: string | undefined;
  amount: string;
  operationDate: string;
  comment: string;
  reminderEnabled: boolean;
  reminderOffsetDays: 1 | 3 | 7;
};

type FormErrors = {
  name?: string;
  amount?: string;
  categoryId?: string;
  operationDate?: string;
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
    categoryId: operation.categoryId,
    amount: formatAmountFromKopecks(operation.amountKopecks),
    operationDate: operation.operationDate,
    comment: operation.comment ?? '',
    reminderEnabled: operation.reminderOffsetDays !== null && operation.reminderOffsetDays !== undefined,
    reminderOffsetDays: operation.reminderOffsetDays ?? 1,
  });
  const [errors, setErrors] = useState<FormErrors>({});

  const hasChanges = useMemo(() => {
    const amountKopecks = parseAmountToKopecks(form.amount);
    const currentReminder = form.reminderEnabled ? form.reminderOffsetDays : null;
    const originalReminder = operation.reminderOffsetDays ?? null;

    return (
      form.name.trim() !== operation.name ||
      form.type !== operation.type ||
      form.categoryId !== operation.categoryId ||
      amountKopecks !== operation.amountKopecks ||
      form.operationDate !== operation.operationDate ||
      (form.comment.trim() || undefined) !== (operation.comment ?? undefined) ||
      currentReminder !== originalReminder
    );
  }, [
    form.name,
    form.type,
    form.categoryId,
    form.amount,
    form.operationDate,
    form.comment,
    form.reminderEnabled,
    form.reminderOffsetDays,
    operation,
  ]);

  const handleTypeChange = (type: OperationType) => {
    setForm((prev) => ({ ...prev, type, categoryId: undefined }));
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
    setForm((prev) => ({ ...prev, operationDate: value ?? '' }));
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

    if (!form.categoryId) {
      next.categoryId = 'Выберите категорию';
    }

    if (!form.operationDate) {
      next.operationDate = 'Выберите дату';
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
    if (amountKopecks === undefined || !form.categoryId) {
      return;
    }

    updateOperation.mutate(
      {
        id,
        propertyId: operation.propertyId ?? undefined,
        data: {
          name: form.name.trim(),
          type: form.type,
          categoryId: form.categoryId,
          amountKopecks,
          operationDate: form.operationDate,
          comment: form.comment.trim() || undefined,
          reminderOffsetDays: form.reminderEnabled ? form.reminderOffsetDays : 0,
        },
      },
      {
        onSuccess: () => {
          goBack(router, ROUTES.financeOperation(id));
        },
        onError: (error) => {
          notify.scenarios.operations.operationSaveError(error);
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
          value={form.categoryId}
          onChange={(categoryId) => setForm((prev) => ({ ...prev, categoryId }))}
          error={errors.categoryId}
          propertyId={operation.propertyId ?? undefined}
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
          value={form.operationDate}
          onChange={handleDateChange}
          required
          error={errors.operationDate}
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
      onClick={() => goBack(router, ROUTES.financeOperation(id))}
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
          operation={data}
          readonly={readonly}
        />
      )}
    </div>
  );
}
