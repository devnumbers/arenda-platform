'use client';

import { useCallback, useMemo, useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { toast } from 'react-toastify';
import {
  useTenantContact,
  useUpdateTenantContact,
} from '@/features/tenant-contacts/api';
import { ApiError } from '@/shared/api/errors';
import { ROUTES } from '@/shared/config/routes';
import { TenantDetailLoading } from '@/widgets/tenant-detail';
import { Button } from '@/shared/ui/button';
import { LinkButton } from '@/shared/ui/link-button';
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

interface TenantContactUpdatePayload {
  name?: string;
  surname?: string | undefined;
  patronymic?: string | undefined;
  phone?: string | undefined;
  email?: string | undefined;
  comment?: string | undefined;
}

function getOptionalFieldChange(
  current: string,
  initial: string | undefined | null,
): string | undefined {
  const normalized = current.trim();
  const normalizedInitial = (initial ?? '').trim();
  if (normalized === normalizedInitial) return undefined;
  return normalized === '' ? undefined : normalized;
}

function mapErrorMessage(error: ApiError): string {
  if (error.status === 409) {
    return 'Арендатор с таким телефоном уже существует';
  }
  if (error.status === 404) {
    return 'Арендатор не найден';
  }
  return error.message;
}

export function TenantEditForm({ tenantId }: TenantEditFormProps): JSX.Element {
  const router = useRouter();
  const [submitError, setSubmitError] = useState<string | undefined>(undefined);

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

      setSubmitError(undefined);

      const payload: TenantContactUpdatePayload = {};

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

      const normalizedPhone =
        data.phone.trim() === '+7' ? '' : data.phone.trim();
      const phoneChange = getOptionalFieldChange(normalizedPhone, tenant.phone);
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
        // Cleared optional fields are omitted from the payload so the backend does not skip them as nil values.
        await updateTenantContact.mutateAsync({
          id: tenantId,
          data: payload as TenantContactUpdateRequest,
        });
        toast.success('Арендатор обновлён');
        router.push(ROUTES.tenant(tenantId));
      } catch (error: unknown) {
        const message =
          error instanceof ApiError
            ? mapErrorMessage(error)
            : error instanceof Error
              ? error.message
              : 'Не удалось сохранить изменения. Попробуйте ещё раз.';
        setSubmitError(message);
        toast.error(message);
      }
    },
    [router, tenant, tenantId, updateTenantContact],
  );

  if (!isValidTenantId) {
    return <TenantEditFormNotFound />;
  }

  if (isPending) {
    return <TenantDetailLoading />;
  }

  if (isError) {
    return (
      <TenantEditFormError onRetry={refetch} isLoading={isFetching} />
    );
  }

  if (!initialData) {
    return <TenantDetailLoading />;
  }

  return (
    <TenantForm
      initialData={initialData}
      submitLabel="Сохранить изменения"
      isLoading={updateTenantContact.isPending}
      error={submitError}
      onSubmit={handleSubmit}
      backHref={ROUTES.tenant(tenantId)}
    />
  );
}
