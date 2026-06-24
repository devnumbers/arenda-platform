'use client';

import { useState, type JSX, type FormEvent } from 'react';
import { useRouter } from 'next/navigation';
import { Select } from '@heroui/react/select';
import { ListBox } from '@heroui/react/list-box';
import { ListBoxItem } from '@heroui/react/list-box-item';
import { Label } from '@heroui/react/label';
import { TextField } from '@/shared/ui/text-field';
import { Button } from '@/shared/ui/button';
import { useCreateProperty } from '@/features/properties/api';
import { propertyTypeOptions } from '@/features/properties/lib/property-types';
import { ROUTES } from '@/shared/config/routes';
import type { PropertyType } from '@/entities/property/model/types';
import styles from './PropertyCreateForm.module.css';

function isPropertyType(value: string): value is PropertyType {
  return propertyTypeOptions.some((option) => option.value === value);
}

export function PropertyCreateForm(): JSX.Element {
  const router = useRouter();
  const create = useCreateProperty();
  const [name, setName] = useState('');
  const [type, setType] = useState<PropertyType | ''>('');
  const [address, setAddress] = useState('');
  const [description, setDescription] = useState('');
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!name || !type || !address) return;
    if (!isPropertyType(type)) return;

    setErrorMessage(null);

    try {
      await create.mutateAsync({
        name,
        type,
        address,
        description,
      });
      router.push(ROUTES.properties);
    } catch (error) {
      setErrorMessage(
        error instanceof Error ? error.message : 'Не удалось создать объект',
      );
    }
  };

  return (
    <form className={styles.root} onSubmit={handleSubmit}>
      <TextField
        label="Название"
        value={name}
        onChange={(e) => setName(e.target.value)}
        required
        fullWidth
      />
      <Select
        value={type || null}
        onChange={(value) =>
          setType(
            typeof value === 'string' && isPropertyType(value) ? value : '',
          )
        }
        isRequired
        placeholder="Выберите тип"
      >
        <Label>Тип объекта</Label>
        <Select.Trigger>
          <Select.Value />
          <Select.Indicator />
        </Select.Trigger>
        <Select.Popover>
          <ListBox>
            {propertyTypeOptions.map((opt) => (
              <ListBoxItem
                key={opt.value}
                id={opt.value}
                textValue={opt.label}
              >
                {opt.label}
              </ListBoxItem>
            ))}
          </ListBox>
        </Select.Popover>
      </Select>
      <TextField
        label="Адрес"
        value={address}
        onChange={(e) => setAddress(e.target.value)}
        required
        fullWidth
      />
      <TextField
        label="Описание"
        value={description}
        onChange={(e) => setDescription(e.target.value)}
        multiline
        fullWidth
      />
      {errorMessage && (
        <p className={styles.error}>{errorMessage}</p>
      )}
      <Button
        type="submit"
        variant="primary"
        size="large"
        fullWidth
        loading={create.isPending}
        disabled={!name || !type || !address || create.isPending}
      >
        Создать объект
      </Button>
    </form>
  );
}
