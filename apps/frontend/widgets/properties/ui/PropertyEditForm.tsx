'use client';

import {
  useEffect,
  useRef,
  useState,
  type FormEvent,
  type JSX,
} from 'react';
import { useRouter } from 'next/navigation';
import { notify } from '@/shared/lib/toast';
import { ROUTES } from '@/shared/config/routes';
import {useProperty, useUpdateProperty} from '@/features/properties/api';
import { TextField } from '@/shared/ui/text-field';
import { Button } from '@/shared/ui/button';
import { PageHeader } from '@/shared/ui/page-header';
import type { PropertyType } from '@/entities/property/model/types';
import { PropertyTypeSelect } from './PropertyTypeSelect';
import { AddressField } from './AddressField';
import styles from './PropertyEditForm.module.css';

const MAX_NAME_LENGTH = 50;
const MAX_DESCRIPTION_LENGTH = 500;

export type PropertyEditFormProps = {
  readonly propertyId: string;
};

type PropertyEditFormErrorProps = {
  readonly onRetry: () => void;
  readonly isLoading?: boolean;
};

function PropertyEditFormError({
  onRetry,
  isLoading = false,
}: PropertyEditFormErrorProps): JSX.Element {
  return (
    <div className={styles.errorCard} role="alert" aria-live="polite">
      <h2 className={styles.errorTitle}>Не удалось загрузить объект</h2>
      <p className={styles.errorMessage}>
        Проверьте соединение и попробуйте снова
      </p>
      <Button
        variant="primary"
        size="medium"
        loading={isLoading}
        onClick={onRetry}
        type="button"
      >
        Повторить
      </Button>
    </div>
  );
}

export function PropertyEditForm({
  propertyId,
}: PropertyEditFormProps): JSX.Element {
  const router = useRouter();

  const propertyQuery = useProperty(propertyId);
  const updateProperty = useUpdateProperty();

  const [type, setType] = useState<PropertyType | undefined>(undefined);
  const [address, setAddress] = useState('');
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [submitAttempted, setSubmitAttempted] = useState(false);
  const hasInitialized = useRef(false);

  useEffect(() => {
    const property = propertyQuery.data;
    if (!property || hasInitialized.current) return;

    // Initialize form state from loaded property data once to avoid wiping
    // user edits on background refetch.
    setType(property.type);
    setAddress(property.address);
    setName(property.name);
    setDescription(property.description ?? '');
    hasInitialized.current = true;
  }, [propertyQuery.data]);

  const isSubmitting = updateProperty.isPending;

  const isNameValid = name.trim().length > 0;
  const isAddressValid = address.trim().length > 0;
  const isTypeValid = type !== undefined;

  const canSubmit =
    isNameValid && isAddressValid && isTypeValid && !isSubmitting;

  const typeError = submitAttempted && !isTypeValid ? 'Выберите тип объекта' : undefined;
  const addressError = submitAttempted && !isAddressValid ? 'Укажите адрес' : undefined;
  const nameError = submitAttempted && !isNameValid ? 'Укажите название' : undefined;

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    setSubmitAttempted(true);

    if (!canSubmit || !type) return;

    try {
      await updateProperty.mutateAsync({
        id: propertyId,
        data: {
          name: name.trim(),
          type,
          address: address.trim(),
          description: description.trim() || undefined,
        },
      });

      notify.success('Объект обновлён');
      router.push(ROUTES.property(propertyId));
    } catch {
      notify.error('Не удалось сохранить изменения');
    }
  };

  if (propertyQuery.isPending) {
    return <div className={styles.loading}>Загрузка...</div>;
  }

  if (propertyQuery.isError || !propertyQuery.data) {
    return (
      <PropertyEditFormError
        onRetry={propertyQuery.refetch}
        isLoading={propertyQuery.isFetching}
      />
    );
  }

  return (
    <form className={styles.root} onSubmit={handleSubmit}>
      <PageHeader title="Информация об объекте" backHref={ROUTES.property(propertyId)} />

      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>Данные</h2>
        <div className={styles.fields}>
          <PropertyTypeSelect value={type} onChange={setType} error={typeError} />
          <AddressField value={address} onChange={setAddress} error={addressError} />
          <TextField
            label="Название"
            required
            fullWidth
            maxLength={MAX_NAME_LENGTH}
            value={name}
            error={nameError}
            onChange={(event) => setName(event.currentTarget.value)}
          />
          <TextField
            label="Описание"
            multiline
            fullWidth
            maxLength={MAX_DESCRIPTION_LENGTH}
            value={description}
            onChange={(event) => setDescription(event.currentTarget.value)}
          />
        </div>
      </section>

      <Button
        type="submit"
        variant="primary"
        size="large"
        fullWidth
        loading={isSubmitting}
        disabled={!canSubmit}
      >
        Сохранить изменения
      </Button>
    </form>
  );
}
