'use client';

import {
  useEffect,
  useId,
  useRef,
  useState,
  type ChangeEvent,
  type JSX,
} from 'react';
import { Button } from '@/shared/ui/button';
import { Icon } from '@/shared/ui/icon';
import { TextField } from '@/shared/ui/text-field';
import { Loading, Trash } from '@/shared/assets/icons';
import type { PropertyType } from '@/entities/property/model/types';
import styles from './PropertyInfoStep.module.css';

const MAX_PHOTO_SIZE = 5 * 1024 * 1024;
const MAX_PHOTO_COUNT = 10;
const ALLOWED_TYPES = ['image/jpeg', 'image/png', 'image/webp'] as const;

export type PropertyInfoStepProps = {
  name?: string;
  description?: string;
  onNameChange: (name: string) => void;
  onDescriptionChange: (description: string) => void;
  draftType?: PropertyType;
  draftAddress?: string;
  photos: File[];
  onPhotosChange: (photos: File[]) => void;
  onSubmit: () => void;
  onBack?: () => void;
  isLoading?: boolean;
};

export function PropertyInfoStep({
  name,
  description,
  onNameChange,
  onDescriptionChange,
  photos,
  onPhotosChange,
  onSubmit,
  isLoading,
}: PropertyInfoStepProps): JSX.Element {
  const fileInputId = useId();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const previewUrlsRef = useRef<string[]>([]);
  const [previews, setPreviews] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    return () => {
      previewUrlsRef.current.forEach((url) => URL.revokeObjectURL(url));
    };
  }, []);

  const validateFiles = (files: File[]): string | null => {
    if (photos.length + files.length > MAX_PHOTO_COUNT) {
      return `Можно загрузить не более ${MAX_PHOTO_COUNT} фотографий`;
    }
    for (const file of files) {
      if (!ALLOWED_TYPES.includes(file.type as (typeof ALLOWED_TYPES)[number])) {
        return 'Поддерживаются форматы JPEG, PNG и WebP';
      }
      if (file.size > MAX_PHOTO_SIZE) {
        return 'Размер каждой фотографии не должен превышать 5 МБ';
      }
    }
    return null;
  };

  const handleAddClick = () => {
    fileInputRef.current?.click();
  };

  const handleFileChange = (event: ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(event.target.files ?? []);
    event.currentTarget.value = '';
    setError(null);

    if (files.length === 0) return;

    const validationError = validateFiles(files);
    if (validationError) {
      setError(validationError);
      return;
    }

    const newUrls = files.map((file) => URL.createObjectURL(file));
    previewUrlsRef.current = [...previewUrlsRef.current, ...newUrls];
    setPreviews((prev) => [...prev, ...newUrls]);
    onPhotosChange([...photos, ...files]);
  };

  const handleRemove = (index: number) => {
    const url = previewUrlsRef.current[index];
    if (url) {
      URL.revokeObjectURL(url);
    }
    previewUrlsRef.current = previewUrlsRef.current.filter((_, i) => i !== index);
    setPreviews((prev) => {
      const next = [...prev];
      next.splice(index, 1);
      return next;
    });
    const nextPhotos = [...photos];
    nextPhotos.splice(index, 1);
    onPhotosChange(nextPhotos);
  };

  const isNameEmpty = (name?.trim() ?? '').length === 0;
  const canSubmit = !isNameEmpty && !isLoading;

  return (
    <div className={styles.root}>
      <h2 className={styles.heading}>Информация об объекте</h2>

      <TextField
        label="Название"
        required
        fullWidth
        value={name ?? ''}
        onChange={(event) => onNameChange(event.currentTarget.value)}
      />

      <TextField
        label="Описание"
        multiline
        fullWidth
        maxLength={500}
        value={description ?? ''}
        onChange={(event) => onDescriptionChange(event.currentTarget.value)}
      />

      <div className={styles.photosSection}>
        <h3 className={styles.photosHeading}>Фотографии</h3>

        <div className={styles.photoGrid}>
          {previews.map((url, index) => (
            <div key={url} className={styles.photoCard}>
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img src={url} alt="" className={styles.photoImage} />
              <button
                type="button"
                className={styles.deleteButton}
                aria-label="Удалить фотографию"
                onClick={() => handleRemove(index)}
              >
                <Icon size="xs">
                  <Trash />
                </Icon>
              </button>
            </div>
          ))}
          {isLoading && (
            <div className={styles.photoLoading}>
              <Icon size="m">
                <Loading />
              </Icon>
            </div>
          )}
        </div>

        {error && <p className={styles.error}>{error}</p>}

        <Button
          type="button"
          variant="secondary"
          size="medium"
          onClick={handleAddClick}
          disabled={photos.length >= MAX_PHOTO_COUNT || isLoading}
        >
          Добавить фото
        </Button>

        <input
          ref={fileInputRef}
          id={fileInputId}
          type="file"
          accept={ALLOWED_TYPES.join(',')}
          multiple
          className={styles.fileInput}
          onChange={handleFileChange}
          aria-hidden="true"
        />
      </div>

      <Button
        type="button"
        variant="primary"
        size="large"
        fullWidth
        loading={isLoading}
        disabled={!canSubmit}
        onClick={onSubmit}
        className={styles.submit}
      >
        Создать объект
      </Button>
    </div>
  );
}
