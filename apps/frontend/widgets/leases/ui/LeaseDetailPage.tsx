'use client';

import {
  useState,
  useMemo,
  useCallback,
  type ChangeEvent,
  type FormEvent,
  type JSX,
} from 'react';
import { useRouter } from 'next/navigation';
import { toast } from 'react-toastify';
import NextLink from 'next/link';
import { ROUTES } from '@/shared/config/routes';
import {
  useLease,
  useUpdateLease,
  useCompleteLease,
  useReturnDeposit,
} from '@/features/leases/api/hooks';
import { useOperations } from '@/features/operations/api/hooks';
import { useProperty } from '@/features/properties/api/hooks';
import { useTenantContacts } from '@/features/tenant-contacts/api/hooks';
import { useSubscription } from '@/features/subscription/api/hooks';
import { isSubscriptionReadonly } from '@/features/subscription/lib/is-subscription-readonly';
import { ApiError } from '@/shared/api/errors';
import { TextField } from '@/shared/ui/text-field';
import { DatePickerField } from '@/shared/ui/date-picker-field';
import { Button } from '@/shared/ui/button';
import { PageHeader } from '@/shared/ui/page-header';
import { PropertyDetailSection } from '@/widgets/property-detail';
import { OperationListItem } from '@/widgets/operations/ui/OperationListItem';
import { StatusBadge } from '@/widgets/dashboard/ui/StatusBadge';
import { FinanceLoading } from '@/widgets/finance/ui/FinanceLoading';
import { FinanceErrorState } from '@/widgets/finance/ui/FinanceErrorState';
import { SubscriptionReadonlyBanner } from '@/widgets/finance/ui/SubscriptionReadonlyBanner';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import type { components } from '@/shared/api/generated';
import type { TenantContact } from '@/entities/tenant-contact/model/types';
import styles from './LeaseDetailPage.module.css';

type LeaseResponse = components['schemas']['LeaseResponse'];
type LeaseUpdateRequest = components['schemas']['LeaseUpdateRequest'];
type OperationResponse = components['schemas']['OperationResponse'];

function formatErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    return error.detail;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return 'Не удалось сохранить изменения. Попробуйте ещё раз.';
}

function kopecksToRubles(kopecks: number): string {
  return (kopecks / 100).toFixed(2);
}

function parseRublesToKopecks(value: string): number | undefined {
  const normalized = value.trim().replace(',', '.');
  if (normalized === '') {
    return undefined;
  }
  const number = Number(normalized);
  if (Number.isNaN(number) || number < 0) {
    return undefined;
  }
  return Math.round(number * 100);
}

function formatDateLabel(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) {
    return iso;
  }
  return date.toLocaleDateString('ru-RU');
}

type FormData = {
  tenantContactId: string;
  startDate: string;
  endDate: string;
  rentAmount: string;
  depositAmount: string;
  paymentDay: string;
  comment: string;
};

type FormErrors = {
  rentAmount?: string;
  depositAmount?: string;
  paymentDay?: string;
  startDate?: string;
  endDate?: string;
};

export type LeaseDetailPageProps = {
  readonly id: string;
};

function formatTenantName(contact: {
  name: string;
  surname: string | null;
  patronymic: string | null;
}): string {
  return [contact.surname, contact.name, contact.patronymic]
    .filter(Boolean)
    .join(' ');
}

function TenantCard({
  tenantContact,
}: {
  readonly tenantContact: LeaseResponse['tenant_contact'];
}): JSX.Element {
  if (!tenantContact) {
    return (
      <PropertyDetailSection>
        <h2 className={styles.sectionTitle}>Арендатор</h2>
        <div className={styles.card}>
          <p className={styles.emptyText}>Арендатор не указан</p>
        </div>
      </PropertyDetailSection>
    );
  }

  const displayName = formatTenantName(tenantContact) || tenantContact.name;

  return (
    <PropertyDetailSection>
      <h2 className={styles.sectionTitle}>Арендатор</h2>
      <NextLink
        href={ROUTES.tenant(tenantContact.id)}
        className={styles.card}
      >
        <p className={styles.detailValue}>{displayName}</p>
        {tenantContact.phone && (
          <p className={styles.detailValue}>{tenantContact.phone}</p>
        )}
        {tenantContact.email && (
          <p className={styles.detailValue}>{tenantContact.email}</p>
        )}
        {tenantContact.comment && (
          <p className={styles.detailComment}>{tenantContact.comment}</p>
        )}
      </NextLink>
    </PropertyDetailSection>
  );
}

function TermsCard({ lease }: { readonly lease: LeaseResponse }): JSX.Element {
  const endDate = lease.end_date ?? null;

  return (
    <PropertyDetailSection>
      <h2 className={styles.sectionTitle}>Условия аренды</h2>
      <div className={styles.card}>
        <div className={styles.detailRow}>
          <dt className={styles.detailLabel}>Начало аренды</dt>
          <dd className={styles.detailValue}>
            {formatDateLabel(lease.start_date)}
          </dd>
        </div>
        <div className={styles.detailRow}>
          <dt className={styles.detailLabel}>Конец аренды</dt>
          <dd className={styles.detailValue}>
            {endDate ? formatDateLabel(endDate) : 'Бессрочно'}
          </dd>
        </div>
        <div className={styles.detailRow}>
          <dt className={styles.detailLabel}>Арендная плата</dt>
          <dd className={styles.detailValue}>
            {formatMoneyKopecks(lease.rent_amount_kopecks)}
          </dd>
        </div>
        <div className={styles.detailRow}>
          <dt className={styles.detailLabel}>День оплаты</dt>
          <dd className={styles.detailValue}>{lease.payment_day}-е число</dd>
        </div>
        <div className={styles.detailRow}>
          <dt className={styles.detailLabel}>Залог</dt>
          <dd className={styles.detailValue}>
            {formatMoneyKopecks(lease.deposit_amount_kopecks)}
          </dd>
        </div>
        {lease.comment && (
          <div className={styles.detailRow}>
            <dt className={styles.detailLabel}>Комментарий</dt>
            <dd className={styles.detailValue}>{lease.comment}</dd>
          </div>
        )}
      </div>
    </PropertyDetailSection>
  );
}

function OperationsSection({
  operations,
  isLoading,
  isError,
  isFetching,
  onRetry,
}: {
  readonly operations: ReadonlyArray<OperationResponse>;
  readonly isLoading: boolean;
  readonly isError: boolean;
  readonly isFetching: boolean;
  readonly onRetry: () => void;
}): JSX.Element {
  if (isLoading) {
    return (
      <PropertyDetailSection>
        <h2 className={styles.sectionTitle}>Арендная плата</h2>
        <div className={styles.card}>
          <FinanceLoading />
        </div>
      </PropertyDetailSection>
    );
  }

  if (isError) {
    return (
      <PropertyDetailSection>
        <h2 className={styles.sectionTitle}>Арендная плата</h2>
        <div className={styles.card}>
          <p className={styles.emptyText}>
            Не удалось загрузить арендные операции.
          </p>
          <Button
            type="button"
            variant="secondary"
            size="medium"
            loading={isFetching}
            onClick={onRetry}
          >
            Повторить
          </Button>
        </div>
      </PropertyDetailSection>
    );
  }

  if (operations.length === 0) {
    return (
      <PropertyDetailSection>
        <h2 className={styles.sectionTitle}>Арендная плата</h2>
        <div className={styles.card}>
          <p className={styles.emptyText}>Арендных операций пока нет</p>
        </div>
      </PropertyDetailSection>
    );
  }

  return (
    <PropertyDetailSection>
      <h2 className={styles.sectionTitle}>Арендная плата</h2>
      <ul className={styles.operationsList}>
        {operations.map((operation) => (
          <li key={operation.id}>
            <OperationListItem operation={operation} />
          </li>
        ))}
      </ul>
    </PropertyDetailSection>
  );
}

function getTenantContactOptionLabel(contact: TenantContact): string {
  return formatTenantName(contact) || contact.name;
}

function LeaseEditForm({
  lease,
  onCancel,
}: {
  readonly lease: LeaseResponse;
  readonly onCancel: () => void;
}): JSX.Element {
  const updateLease = useUpdateLease();
  const { data: tenantContacts } = useTenantContacts();

  const [form, setForm] = useState<FormData>({
    tenantContactId: lease.tenant_contact?.id ?? '',
    startDate: lease.start_date,
    endDate: lease.end_date ?? '',
    rentAmount: kopecksToRubles(lease.rent_amount_kopecks),
    depositAmount: kopecksToRubles(lease.deposit_amount_kopecks),
    paymentDay: String(lease.payment_day),
    comment: lease.comment ?? '',
  });
  const [errors, setErrors] = useState<FormErrors>({});

  const handleTenantChange = (event: ChangeEvent<HTMLSelectElement>) => {
    const tenantContactId = event.currentTarget.value;
    setForm((prev) => ({ ...prev, tenantContactId }));
  };

  const handleChange =
    (field: keyof FormData) =>
    (event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
      const value = event.currentTarget.value;
      setForm((prev) => ({ ...prev, [field]: value }));
    };

  const validate = (): boolean => {
    const next: FormErrors = {};

    const rentKopecks = parseRublesToKopecks(form.rentAmount);
    if (rentKopecks === undefined) {
      next.rentAmount = 'Введите сумму аренды';
    }

    if (
      form.depositAmount.trim() !== '' &&
      parseRublesToKopecks(form.depositAmount) === undefined
    ) {
      next.depositAmount = 'Введите корректную сумму залога';
    }

    const paymentDay = Number(form.paymentDay);
    if (Number.isNaN(paymentDay) || paymentDay < 1 || paymentDay > 31) {
      next.paymentDay = 'Введите число от 1 до 31';
    }

    if (!form.startDate) {
      next.startDate = 'Укажите дату начала';
    }

    if (
      form.endDate &&
      form.startDate &&
      new Date(form.endDate) < new Date(form.startDate)
    ) {
      next.endDate = 'Дата окончания не может быть раньше начала';
    }

    setErrors(next);
    return Object.keys(next).length === 0;
  };

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (!validate()) {
      return;
    }

    const rentKopecks = parseRublesToKopecks(form.rentAmount);
    const depositKopecks = parseRublesToKopecks(form.depositAmount);
    const paymentDay = Number(form.paymentDay);

    if (rentKopecks === undefined || Number.isNaN(paymentDay)) {
      return;
    }

    const data: LeaseUpdateRequest = {
      tenant_contact_id: form.tenantContactId || undefined,
      clear_tenant_contact: !form.tenantContactId,
      start_date: form.startDate,
      ...(form.endDate ? { end_date: form.endDate } : {}),
      rent_amount_kopecks: rentKopecks,
      deposit_amount_kopecks: depositKopecks ?? 0,
      payment_day: paymentDay,
      ...(form.comment.trim() ? { comment: form.comment.trim() } : {}),
    };

    updateLease.mutate(
      { id: lease.id, data },
      {
        onSuccess: onCancel,
        onError: (error) => toast.error(formatErrorMessage(error)),
      },
    );
  };

  return (
    <form className={styles.form} onSubmit={handleSubmit}>
      <PropertyDetailSection>
        <h2 className={styles.sectionTitle}>Редактирование аренды</h2>
        <div className={styles.fields}>
          <div className={styles.selectField}>
            <label htmlFor="tenant-contact" className={styles.selectLabel}>
              Арендатор
            </label>
            <select
              id="tenant-contact"
              className={styles.select}
              value={form.tenantContactId}
              onChange={handleTenantChange}
            >
              <option value="">Не указан</option>
              {tenantContacts?.map((contact) => (
                <option key={contact.id} value={contact.id}>
                  {getTenantContactOptionLabel(contact)}
                </option>
              ))}
            </select>
          </div>

          <DatePickerField
            label="Начало аренды"
            value={form.startDate}
            onChange={(value) => setForm((prev) => ({ ...prev, startDate: value }))}
            required
            fullWidth
            error={errors.startDate}
          />
          <DatePickerField
            label="Конец аренды"
            value={form.endDate}
            onChange={(value) => setForm((prev) => ({ ...prev, endDate: value }))}
            minValue={form.startDate}
            fullWidth
            error={errors.endDate}
          />
          <TextField
            label="Арендная плата, ₽"
            type="number"
            min={0}
            step="0.01"
            required
            fullWidth
            value={form.rentAmount}
            onChange={handleChange('rentAmount')}
            error={errors.rentAmount}
          />
          <TextField
            label="Залог, ₽"
            type="number"
            min={0}
            step="0.01"
            fullWidth
            value={form.depositAmount}
            onChange={handleChange('depositAmount')}
            error={errors.depositAmount}
          />
          <TextField
            label="День оплаты"
            type="number"
            min={1}
            max={31}
            required
            fullWidth
            value={form.paymentDay}
            onChange={handleChange('paymentDay')}
            error={errors.paymentDay}
          />
          <TextField
            label="Комментарий"
            placeholder="Дополнительная информация"
            multiline
            maxLength={500}
            showCounter
            fullWidth
            value={form.comment}
            onChange={handleChange('comment')}
          />
        </div>
      </PropertyDetailSection>

      {updateLease.error && (
        <p className={styles.error} role="alert">
          {formatErrorMessage(updateLease.error)}
        </p>
      )}

      <div className={styles.actions}>
        <Button
          type="submit"
          variant="primary"
          size="large"
          fullWidth
          loading={updateLease.isPending}
        >
          Сохранить
        </Button>
        <Button
          type="button"
          variant="secondary"
          size="large"
          fullWidth
          onClick={onCancel}
        >
          Отмена
        </Button>
      </div>
    </form>
  );
}

export function LeaseDetailPage({ id }: LeaseDetailPageProps): JSX.Element {
  const router = useRouter();
  const [isEditing, setIsEditing] = useState(false);

  const leaseQuery = useLease(id);
  const rentOperationsQuery = useOperations({ lease_id: id, category: 'rent' });
  const depositReturnQuery = useOperations({
    lease_id: id,
    category: 'deposit_return',
    limit: 1,
  });
  const { data: subscription } = useSubscription();
  const readonly = isSubscriptionReadonly(subscription);

  const completeLease = useCompleteLease();
  const returnDeposit = useReturnDeposit();

  const lease = leaseQuery.data;
  const propertyQuery = useProperty(lease?.property_id ?? '');
  const propertyName = propertyQuery.data?.name ?? 'Объект';
  const rentOperations = useMemo(
    () => rentOperationsQuery.data?.items ?? [],
    [rentOperationsQuery.data],
  );
  const hasDepositReturn = useMemo(
    () => (depositReturnQuery.data?.items ?? []).length > 0,
    [depositReturnQuery.data],
  );

  const isLoading = leaseQuery.isPending;
  const isError = leaseQuery.isError;
  const isFetching = leaseQuery.isFetching;
  const canCompleteLease =
    lease !== undefined &&
    !readonly &&
    (lease.status === 'active' || lease.status === 'requires_action');
  const canReturnDeposit =
    lease !== undefined &&
    !readonly &&
    !depositReturnQuery.isPending &&
    !depositReturnQuery.isError &&
    (lease.status === 'completed' || lease.status === 'requires_action') &&
    lease.deposit_amount_kopecks > 0 &&
    !hasDepositReturn;
  const showLeaseActions = canCompleteLease || canReturnDeposit;

  const handleRetry = useCallback(() => {
    if (leaseQuery.isError) {
      leaseQuery.refetch();
    }
  }, [leaseQuery]);

  const handleOperationsRetry = useCallback(() => {
    rentOperationsQuery.refetch();
  }, [rentOperationsQuery]);

  const handleToggleEdit = useCallback(() => {
    setIsEditing((prev) => !prev);
  }, []);

  const handleComplete = useCallback(() => {
    if (!confirm('Завершить аренду?')) {
      return;
    }
    completeLease.mutate(id, {
      onError: (error) => toast.error(formatErrorMessage(error)),
    });
  }, [completeLease, id]);

  const handleReturnDeposit = useCallback(() => {
    if (!confirm('Вернуть залог?')) {
      return;
    }
    returnDeposit.mutate(id, {
      onError: (error) => toast.error(formatErrorMessage(error)),
    });
  }, [returnDeposit, id]);

  if (!id) {
    return (
      <FinanceErrorState
        onRetry={() => router.push(ROUTES.properties)}
        isLoading={false}
      />
    );
  }

  const title = lease ? (
    <NextLink
      href={ROUTES.property(lease.property_id)}
      className={styles.titleLink}
    >
      <span className={styles.title}>{propertyName}</span>
      <StatusBadge status={lease.status} />
    </NextLink>
  ) : null;

  return (
    <div className={styles.root}>
      <SubscriptionReadonlyBanner />

      {isLoading && <FinanceLoading />}

      {!isLoading && isError && (
        <FinanceErrorState onRetry={handleRetry} isLoading={isFetching} />
      )}

      {!isLoading && !isError && lease && (
        <>
          <PageHeader
            title={title}
            backHref={ROUTES.property(lease.property_id)}
            actions={
              !readonly && (
                <Button
                  type="button"
                  variant="primary"
                  size="small"
                  onClick={handleToggleEdit}
                >
                  {isEditing ? 'Отмена' : 'Редактировать'}
                </Button>
              )
            }
          />

          {isEditing ? (
            <LeaseEditForm lease={lease} onCancel={handleToggleEdit} />
          ) : (
            <>
              <TenantCard tenantContact={lease.tenant_contact} />
              <TermsCard lease={lease} />

              {showLeaseActions && (
                <div className={styles.actions}>
                  {canCompleteLease && (
                    <Button
                      type="button"
                      variant="secondary"
                      size="large"
                      fullWidth
                      loading={completeLease.isPending}
                      onClick={handleComplete}
                    >
                      Завершить аренду
                    </Button>
                  )}
                  {canReturnDeposit && (
                    <Button
                      type="button"
                      variant="secondary"
                      size="large"
                      fullWidth
                      loading={returnDeposit.isPending}
                      onClick={handleReturnDeposit}
                    >
                      Вернуть залог
                    </Button>
                  )}
                </div>
              )}

              <OperationsSection
                operations={rentOperations}
                isLoading={rentOperationsQuery.isPending}
                isError={rentOperationsQuery.isError}
                isFetching={rentOperationsQuery.isFetching}
                onRetry={handleOperationsRetry}
              />
            </>
          )}
        </>
      )}
    </div>
  );
}
