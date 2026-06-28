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
import { useOperationsByLease } from '@/features/operations/api/hooks';
import { useProperty } from '@/features/properties/api/hooks';
import { useTenantContacts } from '@/features/tenant-contacts/api/hooks';
import { useSubscription } from '@/features/subscription/api/hooks';
import { isSubscriptionReadonly } from '@/features/subscription/lib/is-subscription-readonly';
import { ApiError } from '@/shared/api/errors';
import { TextField } from '@/shared/ui/text-field';
import { Button } from '@/shared/ui/button';
import { IconLink } from '@/shared/ui/icon-link';
import { Icon } from '@/shared/ui/icon';
import { ArrowLeft } from '@/shared/assets/icons';
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

function LeaseDetailHeader({
  lease,
  isEditing,
  readonly,
  onToggleEdit,
}: {
  readonly lease: LeaseResponse;
  readonly isEditing: boolean;
  readonly readonly: boolean;
  readonly onToggleEdit: () => void;
}): JSX.Element {
  const propertyQuery = useProperty(lease.property_id);
  const propertyName = propertyQuery.data?.name ?? 'Объект';

  return (
    <header className={styles.header}>
      <IconLink
        href={ROUTES.property(lease.property_id)}
        aria-label="Назад"
        icon={
          <Icon size="m">
            <ArrowLeft />
          </Icon>
        }
      />
      <NextLink
        href={ROUTES.property(lease.property_id)}
        className={styles.headerMain}
      >
        <h1 className={styles.title}>{propertyName}</h1>
        <StatusBadge status={lease.status} />
      </NextLink>
      {!readonly && (
        <Button
          type="button"
          variant="secondary"
          size="small"
          onClick={onToggleEdit}
        >
          {isEditing ? 'Отмена' : 'Редактировать'}
        </Button>
      )}
    </header>
  );
}

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
}: {
  readonly operations: ReadonlyArray<OperationResponse>;
}): JSX.Element {
  if (operations.length === 0) {
    return (
      <PropertyDetailSection>
        <h2 className={styles.sectionTitle}>Операции</h2>
        <div className={styles.card}>
          <p className={styles.emptyText}>Операций пока нет</p>
        </div>
      </PropertyDetailSection>
    );
  }

  return (
    <PropertyDetailSection>
      <h2 className={styles.sectionTitle}>Операции</h2>
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
    setForm((prev) => ({ ...prev, tenantContactId: event.currentTarget.value }));
  };

  const handleChange =
    (field: keyof FormData) =>
    (event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
      setForm((prev) => ({ ...prev, [field]: event.currentTarget.value }));
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

          <TextField
            label="Начало аренды"
            type="date"
            required
            fullWidth
            value={form.startDate}
            onChange={handleChange('startDate')}
            error={errors.startDate}
          />
          <TextField
            label="Конец аренды"
            type="date"
            fullWidth
            value={form.endDate}
            onChange={handleChange('endDate')}
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
  const operationsQuery = useOperationsByLease(id);
  const { data: subscription } = useSubscription();
  const readonly = isSubscriptionReadonly(subscription);

  const completeLease = useCompleteLease();
  const returnDeposit = useReturnDeposit();

  const lease = leaseQuery.data;
  const operations = useMemo(
    () => operationsQuery.data?.items ?? [],
    [operationsQuery.data],
  );

  const isLoading = leaseQuery.isPending || operationsQuery.isPending;
  const isError = leaseQuery.isError || operationsQuery.isError;
  const isFetching = leaseQuery.isFetching || operationsQuery.isFetching;

  const handleRetry = useCallback(() => {
    if (leaseQuery.isError) {
      leaseQuery.refetch();
    }
    if (operationsQuery.isError) {
      operationsQuery.refetch();
    }
  }, [leaseQuery, operationsQuery]);

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

  return (
    <div className={styles.root}>
      <SubscriptionReadonlyBanner />

      {isLoading && <FinanceLoading />}

      {!isLoading && isError && (
        <FinanceErrorState onRetry={handleRetry} isLoading={isFetching} />
      )}

      {!isLoading && !isError && lease && (
        <>
          <LeaseDetailHeader
            lease={lease}
            isEditing={isEditing}
            readonly={readonly}
            onToggleEdit={handleToggleEdit}
          />

          {isEditing ? (
            <LeaseEditForm lease={lease} onCancel={handleToggleEdit} />
          ) : (
            <>
              <TenantCard tenantContact={lease.tenant_contact} />
              <TermsCard lease={lease} />

              {lease.status !== 'completed' && lease.status !== 'archived' && (
                <div className={styles.actions}>
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
                </div>
              )}

              <OperationsSection operations={operations} />
            </>
          )}
        </>
      )}
    </div>
  );
}
