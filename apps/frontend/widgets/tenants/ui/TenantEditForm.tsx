'use client';

import { useCallback, useMemo, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { notify } from '@/shared/lib/notifications';
import {
  useTenantContact,
  useUpdateTenantContact,
} from '@/features/tenant-contacts/api';
import { Skeleton } from '@heroui/react/skeleton';
import { ROUTES } from '@/shared/config/routes';
import { Button } from '@/shared/ui/button';
import { LinkButton } from '@/shared/ui/link-button';
import { PageHeader } from '@/shared/ui/page-header';
import type { components } from '@/shared/api/generated';
import { TenantForm, type TenantContactFormData } from './TenantForm';
import styles from './TenantEditForm.module.css';

const UUID_REGEX = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

type TenantContactUpdateRequest = components['schemas']['TenantContactUpdateRequest'];

export type TenantEditFormProps = {
  readonly tenantId: string;
};

function TenantEditFormNotFound(): JSX.Element {
  return (
    <div className={styles.errorState} role="alert" aria-live="polite">
      <p className={styles.errorText}>Арендатор не найден</p>
      <LinkButton href={ROUTES.tenants} variant="primary" size="medium">
        К списку арендаторов
      </LinkButton>
    </div>
  );
}

type TenantEditFormErrorProps = {
  readonly onRetry: () => void;
  readonly isLoading?: boolean;
};

function TenantEditFormError({
  onRetry,
  isLoading = false,
}: TenantEditFormErrorProps): JSX.Element {
  return (
    <div className={styles.errorState} role="alert" aria-live="polite">
      <p className={styles.errorText}>Не удалось загрузить данные арендатора</p>
      <Button
        type="button"
        variant="primary"
        size="medium"
        loading={isLoading}
        onClick={onRetry}
      >
        Повторить
      </Button>
    </div>
  );
}

function TenantEditLoading(): JSX.Element {
  return (
    <div className={styles.loading} role="status" aria-busy="true" aria-label="Загрузка формы арендатора">
      <Skeleton className={styles.skeletonField} />
      <Skeleton className={styles.skeletonField} />
      <Skeleton className={styles.skeletonField} />
      <Skeleton className={styles.skeletonButton} />
    </div>
  );
}

function getOptionalFieldChange(
  current: string,
  initial: string | undefined | null,
): string | undefined {
  const normalized = current.trim();
  const normalizedInitial = (initial ?? '').trim();
  if (normalized === normalizedInitial) return undefined;
  // Return an empty string to tell the backend to clear the optional field;
  // omitted fields are left unchanged.
  return normalized;
}

export function TenantEditForm({ tenantId }: TenantEditFormProps): JSX.Element {
  const router = useRouter();

  const isValidTenantId = useMemo(
    () => UUID_REGEX.test(tenantId),
    [tenantId],
  );

  const {
    data: tenant,
    isPending,
    isError,
    isFetching,
    refetch,
  } = useTenantContact(isValidTenantId ? tenantId : '');

  const updateTenantContact = useUpdateTenantContact();

  const initialData: Partial<TenantContactFormData> | undefined = useMemo(() => {
    if (!tenant) {
      return undefined;
    }
    return {
      name: tenant.name,
      surname: tenant.surname ?? '',
      patronymic: tenant.patronymic ?? '',
      phone: tenant.phone ?? '',
      email: tenant.email ?? '',
      comment: tenant.comment ?? '',
    };
  }, [tenant]);

  const handleSubmit = useCallback(
    async (data: TenantContactFormData) => {
      if (!tenant) {
        return;
      }

      const payload: TenantContactUpdateRequest = {};

      const normalizedName = data.name.trim();
      if (normalizedName !== tenant.name.trim()) {
        payload.name = normalizedName;
      }

      const surnameChange = getOptionalFieldChange(data.surname, tenant.surname);
      if (surnameChange !== undefined) {
        payload.surname = surnameChange;
      }

      const patronymicChange = getOptionalFieldChange(data.patronymic, tenant.patronymic);
      if (patronymicChange !== undefined) {
        payload.patronymic = patronymicChange;
      }

      const phoneChange = getOptionalFieldChange(data.phone.trim(), tenant.phone);
      if (phoneChange !== undefined) {
        payload.phone = phoneChange;
      }

      const emailChange = getOptionalFieldChange(data.email, tenant.email);
      if (emailChange !== undefined) {
        payload.email = emailChange;
      }

      const commentChange = getOptionalFieldChange(data.comment, tenant.comment);
      if (commentChange !== undefined) {
        payload.comment = commentChange;
      }

      try {
        // Optional fields are sent as empty strings to request clearing them on the backend.
        // Unchanged fields remain omitted and are left as-is.
        await updateTenantContact.mutateAsync({
          id: tenantId,
          data: payload,
        });
        notify.scenarios.tenants.tenantUpdated();
        router.push(ROUTES.tenant(tenantId));
      } catch (error: unknown) {
        notify.scenarios.tenants.tenantUpdateError(error);
      }
    },
    [router, tenant, tenantId, updateTenantContact],
  );

  if (!isValidTenantId) {
    return <TenantEditFormNotFound />;
  }

  return (
    <>
      <PageHeader
        title="Редактирование арендатора"
        backHref={ROUTES.tenant(tenantId)}
      />

      {isPending && <TenantEditLoading />}

      {!isPending && isError && (
        <TenantEditFormError onRetry={refetch} isLoading={isFetching} />
      )}

      {!isPending && !isError && !initialData && <TenantEditFormNotFound />}

      {!isPending && !isError && initialData && (
        <TenantForm
          initialData={initialData}
          submitLabel="Сохранить изменения"
          isLoading={updateTenantContact.isPending}
          onSubmit={handleSubmit}
          backHref={ROUTES.tenant(tenantId)}
        />
      )}
    </>
  );
}
