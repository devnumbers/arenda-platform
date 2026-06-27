'use client';

import {
  type ChangeEvent,
  type FormEvent,
  useEffect,
  useRef,
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
import styles from './OperationEditForm.module.css';

type FormData = {
  name: string;
  type: OperationType;
  category: OperationCategory | undefined;
  amount: string;
  operation_date: string;
  comment: string;
};

type FormErrors = {
  name?: string;
  amount?: string;
  category?: string;
  operation_date?: string;
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
  readonly operation: {
    readonly id: string;
    readonly property_id: string;
    readonly name: string;
    readonly type: OperationType;
    readonly category: OperationCategory;
    readonly amount_kopecks: number;
    readonly operation_date: string;
    readonly comment?: string | null;
    readonly lease_id?: string | null;
  };
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

  const handleDateChange = (event: ChangeEvent<HTMLInputElement>) => {
    setForm((prev) => ({ ...prev, operation_date: event.currentTarget.value }));
  };

  const handleCommentChange = (event: ChangeEvent<HTMLTextAreaElement>) => {
    setForm((prev) => ({ ...prev, comment: event.currentTarget.value }));
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
        },
      },
      {
        onSuccess: () => {
          router.push(ROUTES.financeOperation(id));
        },
      },
    );
  };

  const handleCancel = () => {
    router.push(ROUTES.financeOperation(id));
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
        <TextField
          label="Дата операции"
          type="date"
          required
          fullWidth
          value={form.operation_date}
          onChange={handleDateChange}
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
      </div>

      {operation.lease_id && (
        <p className={styles.note}>
          Эта операция создана из аренды. После изменения она станет исключением.
        </p>
      )}

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

export function OperationEditForm(): JSX.Element {
  const id = useOperationId();
  const router = useRouter();
  const { data, isLoading, isError, refetch, isFetching } = useOperation(id ?? '');
  const { data: subscription } = useSubscription();
  const readonly = isSubscriptionReadonly(subscription);
  const initialized = useRef(false);

  useEffect(() => {
    if (data) {
      initialized.current = true;
    }
  }, [data]);

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
      <header className={styles.header}>
        <IconLink
          href={ROUTES.financeOperation(id)}
          variant="icon-black"
          size="medium"
          icon={
            <Icon size="m">
              <ArrowLeft />
            </Icon>
          }
          aria-label="Назад к операции"
        />
        <h1 className={styles.title}>Редактирование операции</h1>
      </header>

      <SubscriptionReadonlyBanner />

      {isLoading && <FinanceLoading />}

      {!isLoading && isError && (
        <FinanceErrorState onRetry={refetch} isLoading={isFetching} />
      )}

      {!isLoading && !isError && data && (
        <OperationEditFormContent id={id} operation={data} readonly={readonly} />
      )}
    </div>
  );
}
