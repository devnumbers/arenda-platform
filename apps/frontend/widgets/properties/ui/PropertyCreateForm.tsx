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
import styles from './PropertyCreateForm.module.css';

export function PropertyCreateForm(): JSX.Element {
  const router = useRouter();
  const create = useCreateProperty();
  const [name, setName] = useState('');
  const [type, setType] = useState<string>('');
  const [address, setAddress] = useState('');
  const [description, setDescription] = useState('');

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!name || !type) return;
    await create.mutateAsync({
      name,
      type: type as never,
      address,
      description,
    });
    router.push(ROUTES.properties);
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
        selectedKey={type}
        onSelectionChange={(key) => setType((key as string) ?? '')}
        isRequired
      >
        <Label>Тип объекта</Label>
        <Select.Trigger>
          <Select.Value />
        </Select.Trigger>
        <Select.Popover>
          <ListBox>
            {propertyTypeOptions.map((opt) => (
              <ListBoxItem key={opt.value} textValue={opt.label}>
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
        fullWidth
      />
      <TextField
        label="Описание"
        value={description}
        onChange={(e) => setDescription(e.target.value)}
        multiline
        fullWidth
      />
      <Button
        type="submit"
        variant="primary"
        size="large"
        fullWidth
        loading={create.isPending}
      >
        Создать объект
      </Button>
    </form>
  );
}
