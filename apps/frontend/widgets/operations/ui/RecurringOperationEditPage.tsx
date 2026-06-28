'use client';

import {
  type ChangeEvent,
  type FormEvent,
  useState,
  type JSX,
} from 'react';
import { useParams, useRouter } from 'next/navigation';
import { ArrowLeft } from '@/shared/assets/icons';
import { Icon } from '@/shared/ui/icon';
import { IconLink } from '@/shared/ui/icon-link';
import { Button } from '@/shared/ui/button';
import { TextField } from '@/shared/ui/text-field';
import { ROUTES } from '@/shared/config/routes';
import { ApiError } from '@/shared/api/errors';
import type { components } from '@/shared/api/generated';
import {
  type OperationCategory,
  type OperationFrequency,
  type OperationType,
} from '@/entities/operation/model/types';
import { getCategoriesByType } from '@/entities/operation/lib/categories';
import {
  useRecurringOperation,
  useUpdateRecurringOperation,
} from '@/features/recurring-operations/api/hooks';
import { FinanceLoading } from '@/widgets/finance/ui/FinanceLoading';
import { FinanceErrorState } from '@/widgets/finance/ui/FinanceErrorState';
import { SubscriptionReadonlyBanner } from '@/widgets/finance/ui/SubscriptionReadonlyBanner';
import { useSubscription } from '@/features/subscription/api/hooks';
import { isSubscriptionReadonly } from '@/features/subscription/lib/is-subscription-readonly';
import { CategorySelect } from './CategorySelect';
import { TypeSelect } from './TypeSelect';
import { FrequencySelect } from './FrequencySelect';
import styles from './RecurringOperationEditPage.module.css';

type RecurringOperationResponse =
  components['schemas']['RecurringOperationResponse'];
type RecurringOperationUpdateRequest =
  components['schemas']['RecurringOperationUpdateRequest'];

type FormData = {
  type: OperationType;
  category: OperationCategory | undefined;
  name: string;
  amount: string;
  periodicity: OperationFrequency;
  paymentDay: number | undefined;
  startDate: string;
  endDate: string;
  comment: string;
  applyFromDate: string;
};

type FormErrors = {
  name?: string;
  amount?: string;
  category?: string;
  periodicity?: string;
  paymentDay?: string;
  endDate?: string;
  applyFromDate?: string;
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

function parseLocalDate(value: string): Date {
  const [year, month, day] = value.split('-').map(Number);
  return new Date(year, month - 1, day);
}

function formatLocalDate(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

function startOfTodayLocal(): Date {
  const now = new Date();
  return new Date(now.getFullYear(), now.getMonth(), now.getDate());
}

function getInitialApplyFromDate(startDate: string): string {
  const today = startOfTodayLocal();
  const start = parseLocalDate(startDate);
  return formatLocalDate(start > today ? start : today);
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

function useRecurringOperationId(): string | undefined {
  const params = useParams<{ readonly id: string }>();
  return params?.id;
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

  const [form, setForm] = useState<FormData>({
    type: operation.type,
    category: operation.category,
    name: operation.name,
    amount: formatAmountFromKopecks(operation.amount_kopecks),
    periodicity: operation.periodicity,
    paymentDay: operation.payment_day,
    startDate: operation.start_date,
    endDate: operation.end_date ?? '',
    comment: operation.comment ?? '',
    applyFromDate: getInitialApplyFromDate(operation.start_date),
  });
  const [errors, setErrors] = useState<FormErrors>({});

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
    setForm((prev) => ({ ...prev, name: event.currentTarget.value }));
  };

  const handleAmountChange = (event: ChangeEvent<HTMLInputElement>) => {
    setForm((prev) => ({ ...prev, amount: event.currentTarget.value }));
  };

  const handlePaymentDayChange = (event: ChangeEvent<HTMLInputElement>) => {
    const value = event.currentTarget.value;
    setForm((prev) => ({
      ...prev,
      paymentDay: value === '' ? undefined : Number(value),
    }));
  };

  const handleStartDateChange = (event: ChangeEvent<HTMLInputElement>) => {
    setForm((prev) => ({ ...prev, startDate: event.currentTarget.value }));
  };

  const handleEndDateChange = (event: ChangeEvent<HTMLInputElement>) => {
    setForm((prev) => ({ ...prev, endDate: event.currentTarget.value }));
  };

  const handleCommentChange = (event: ChangeEvent<HTMLTextAreaElement>) => {
    setForm((prev) => ({ ...prev, comment: event.currentTarget.value }));
  };

  const handleApplyFromDateChange = (event: ChangeEvent<HTMLInputElement>) => {
    setForm((prev) => ({ ...prev, applyFromDate: event.currentTarget.value }));
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

    if (!form.periodicity) {
      next.periodicity = 'Выберите периодичность';
    }

    if (form.paymentDay === undefined) {
      next.paymentDay = 'Укажите день оплаты';
    } else if (form.paymentDay < 1 || form.paymentDay > 31) {
      next.paymentDay = 'День оплаты должен быть от 1 до 31';
    }

    if (!form.applyFromDate) {
      next.applyFromDate = 'Выберите дату применения изменений';
    }

    if (form.endDate && form.applyFromDate && form.endDate < form.applyFromDate) {
      next.endDate = 'Дата окончания не может быть раньше даты применения';
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
    if (
      amountKopecks === undefined ||
      !form.category ||
      !form.periodicity ||
      form.paymentDay === undefined ||
      !form.applyFromDate
    ) {
      return;
    }

    const data: RecurringOperationUpdateRequest = {
      type: form.type,
      category: form.category,
      name: form.name.trim(),
      amount_kopecks: amountKopecks,
      periodicity: form.periodicity as 'monthly' | 'yearly',
      payment_day: form.paymentDay,
      apply_from_date: form.applyFromDate,
    };

    if (form.endDate) {
      data.end_date = form.endDate;
    }

    if (form.comment.trim()) {
      data.comment = form.comment.trim();
    }

    // start_date cannot be sent together with apply_from_date on the backend.
    if (!form.applyFromDate && form.startDate) {
      data.start_date = form.startDate;
    }

    updateOperation.mutate(
      {
        id,
        propertyId: operation.property_id,
        data,
      },
      {
        onSuccess: () => {
          router.push(`${ROUTES.financeOperations}?tab=recurring`);
        },
      },
    );
  };

  const handleCancel = () => {
    router.push(`${ROUTES.financeOperations}?tab=recurring`);
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
        <FrequencySelect
          value={form.periodicity}
          onChange={(periodicity) => setForm((prev) => ({ ...prev, periodicity }))}
          error={errors.periodicity}
          disabled={readonly}
        />
        <TextField
          label="День оплаты"
          placeholder="1–31"
          type="number"
          min={1}
          max={31}
          required
          fullWidth
          disabled={readonly}
          value={form.paymentDay ?? ''}
          onChange={handlePaymentDayChange}
          error={errors.paymentDay}
        />
        <TextField
          label="Дата начала"
          type="date"
          fullWidth
          disabled={readonly}
          value={form.startDate}
          onChange={handleStartDateChange}
          helperText="Используется, только если не задана дата применения"
        />
        <TextField
          label="Дата окончания (необязательно)"
          type="date"
          fullWidth
          disabled={readonly}
          value={form.endDate}
          onChange={handleEndDateChange}
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
        <TextField
          label="Применить с даты"
          type="date"
          required
          fullWidth
          disabled={readonly}
          value={form.applyFromDate}
          onChange={handleApplyFromDateChange}
          error={errors.applyFromDate}
          helperText="Старые операции до этой даты останутся без изменений, новые создадутся с обновлёнными параметрами."
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
          disabled={readonly}
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
  const { data: subscription } = useSubscription();
  const readonly = isSubscriptionReadonly(subscription);

  if (!id) {
    return (
      <div className={styles.root}>
        <FinanceErrorState
          onRetry={() => router.push(`${ROUTES.financeOperations}?tab=recurring`)}
          isLoading={false}
        />
      </div>
    );
  }

  return (
    <div className={styles.root}>
      <header className={styles.header}>
        <IconLink
          href={`${ROUTES.financeOperations}?tab=recurring`}
          variant="icon-black"
          size="medium"
          icon={
            <Icon size="m">
              <ArrowLeft />
            </Icon>
          }
          aria-label="Назад к регулярным операциям"
        />
        <h1 className={styles.title}>Редактирование серии</h1>
      </header>

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
