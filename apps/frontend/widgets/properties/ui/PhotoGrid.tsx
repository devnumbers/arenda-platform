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
import { Trash } from '@/shared/assets/icons';
import type { PropertyPhoto } from '@/entities/property/model/types';
import styles from './PhotoGrid.module.css';

export const MAX_PHOTO_COUNT = 10;
export const MAX_PHOTO_SIZE = 5 * 1024 * 1024;
export const ALLOWED_TYPES = ['image/jpeg', 'image/png', 'image/webp'] as const;

type PreviewEntry = {
  readonly file: File;
  readonly url: string;
};

export type PhotoGridProps = {
  readonly existingPhotos: PropertyPhoto[];
  readonly newPhotos: File[];
  readonly onNewPhotosChange: (photos: File[]) => void;
  readonly onDeleteExisting: (photoId: string) => void;
  readonly isUploading?: boolean;
};

export function PhotoGrid({
  existingPhotos,
  newPhotos,
  onNewPhotosChange,
  onDeleteExisting,
  isUploading = false,
}: PhotoGridProps): JSX.Element {
  const fileInputId = useId();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [previewEntries, setPreviewEntries] = useState<PreviewEntry[]>([]);
  const [error, setError] = useState<string | null>(null);
  const entriesRef = useRef(previewEntries);

  useEffect(() => {
    entriesRef.current = previewEntries;
  }, [previewEntries]);

  useEffect(() => {
    // Sync preview entries with the new photos controlled by the parent form.
    /* eslint-disable react-hooks/set-state-in-effect */
    setPreviewEntries((prev) => {
      const currentFiles = new Set(newPhotos);
      const kept = prev.filter((entry) => currentFiles.has(entry.file));
      const keptFiles = new Set(kept.map((entry) => entry.file));

      prev.forEach((entry) => {
        if (!currentFiles.has(entry.file)) {
          URL.revokeObjectURL(entry.url);
        }
      });

      const added = newPhotos
        .filter((file) => !keptFiles.has(file))
        .map((file) => ({ file, url: URL.createObjectURL(file) }));

      return [...kept, ...added];
    });
    /* eslint-enable react-hooks/set-state-in-effect */
  }, [newPhotos]);

  useEffect(() => {
    return () => {
      entriesRef.current.forEach((entry) => URL.revokeObjectURL(entry.url));
    };
  }, []);

  const totalCount = existingPhotos.length + newPhotos.length;

  const validateFiles = (files: File[]): string | null => {
    if (totalCount + files.length > MAX_PHOTO_COUNT) {
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

    onNewPhotosChange([...newPhotos, ...files]);
  };

  const handleDeleteExisting = (photoId: string) => {
    onDeleteExisting(photoId);
  };

  const handleDeleteNew = (file: File) => {
    const entry = previewEntries.find((item) => item.file === file);
    if (entry) {
      URL.revokeObjectURL(entry.url);
      setPreviewEntries((prev) => prev.filter((item) => item.file !== file));
    }
    onNewPhotosChange(newPhotos.filter((photo) => photo !== file));
  };

  const isMaxReached = totalCount >= MAX_PHOTO_COUNT;

  return (
    <div className={styles.root}>
      <div className={styles.photoGrid}>
        {existingPhotos.map((photo) => (
          <div key={photo.id} className={styles.photoCard}>
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img
              src={photo.url}
              alt=""
              className={styles.photoImage}
              loading="lazy"
            />
            <button
              type="button"
              className={styles.deleteButton}
              aria-label="Удалить фотографию"
              onClick={() => handleDeleteExisting(photo.id)}
              disabled={isUploading}
            >
              <Icon size="xs">
                <Trash />
              </Icon>
            </button>
          </div>
        ))}

        {previewEntries.map((entry) => (
          <div key={entry.url} className={styles.photoCard}>
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img src={entry.url} alt="" className={styles.photoImage} />
            <button
              type="button"
              className={styles.deleteButton}
              aria-label="Удалить фотографию"
              onClick={() => handleDeleteNew(entry.file)}
              disabled={isUploading}
            >
              <Icon size="xs">
                <Trash />
              </Icon>
            </button>
          </div>
        ))}
      </div>

      {error && <p className={styles.error}>{error}</p>}

      <Button
        type="button"
        variant="secondary"
        size="medium"
        onClick={handleAddClick}
        disabled={isMaxReached || isUploading}
        subtitle={isMaxReached ? `${totalCount} из ${MAX_PHOTO_COUNT} фото` : undefined}
      >
        {isMaxReached ? 'Загружено максимум' : 'Добавить фото'}
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
  );
}
