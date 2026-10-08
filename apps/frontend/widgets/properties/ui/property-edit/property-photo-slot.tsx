'use client';

import { useRef, useState, type ChangeEvent, type JSX } from 'react';
import { notify } from '@/shared/lib/notifications';
import {
  PHOTO_FILE_TOO_LARGE_MESSAGE,
  photoDisplayUrl,
  photoFileTooLarge,
  useDeletePropertyPhoto,
  usePropertyPhotoBuster,
  useUploadPropertyPhoto,
} from '@/features/properties';
import { PropertyAvatar, type PropertyType } from '@/entities/property';
import { Button, ConfirmDialog } from '@/shared/ui/design';

/**
 * Слот фото объекта на правке (Figma 1550:95852, тикет #1228; ADR 0065):
 * круг-плейсхолдер по типу (поверхность hero канона PropertyAvatar) либо
 * загруженное фото, под ним кнопки «Добавить фото»/«Заменить фото» и
 * «Удалить фото». Фото живёт мимо черновика формы — загрузка/удаление
 * применяются сразу отдельными эндпоинтами (`/properties/{id}/photo`,
 * multipart POST и DELETE), кнопки «Сохранить» не касаются. Ошибки —
 * mutateAsync + catch с тостом (канон форм; пер-колбэки mutate не
 * используются).
 *
 * Байты выдачи кэшируются браузером (private, max-age=300) при неизменном
 * пути — URL для <img> собирается с бастером `?v=N` (photoDisplayUrl);
 * счётчик N живёт в react-query кэше (usePropertyPhotoBuster, пишут
 * мутации), поэтому переживает перемонтирования экрана.
 *
 * Кегль кнопок — R/500 16 по макету слота (Figma 1550:95895), поверх
 * канона Clear (кегль M в обоих размерах, Figma 1134:55051) — решение
 * экрана, не снос канона.
 */

const PHOTO_INPUT_ACCEPT = 'image/jpeg,image/png,image/webp';

export type PropertyPhotoSlotProps = {
  readonly propertyId: string;
  /** same-origin стриминговый путь выдачи (ADR 0065), null — фото нет. */
  readonly photoUrl?: string | null;
  /** Тип объекта — выбирает глиф-плейсхолдер, следует за черновиком формы. */
  readonly type?: PropertyType;
};

export function PropertyPhotoSlot({
  propertyId,
  photoUrl,
  type,
}: PropertyPhotoSlotProps): JSX.Element {
  const uploadPhoto = useUploadPropertyPhoto();
  const deletePhoto = useDeletePropertyPhoto();
  const photoBuster = usePropertyPhotoBuster(propertyId);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const displayUrl = photoUrl !== null && photoUrl !== undefined
    ? photoDisplayUrl(photoUrl, photoBuster)
    : null;
  const hasPhoto = displayUrl !== null;

  const openPicker = (): void => {
    fileInputRef.current?.click();
  };

  const handleFileChange = (event: ChangeEvent<HTMLInputElement>): void => {
    const file = event.currentTarget.files?.[0];
    // Сброс значения: повторный выбор того же файла снова зажигает change.
    event.currentTarget.value = '';
    if (file === undefined) {
      return;
    }
    // Пречек капа до сети; формат (магические байты) и пиксельный лимит
    // решает бэкенд, его problem+json текст виден в тосте отказа.
    if (photoFileTooLarge(file.size)) {
      notify.error(PHOTO_FILE_TOO_LARGE_MESSAGE);
      return;
    }
    void uploadPhoto
      .mutateAsync({ id: propertyId, file })
      .catch((error: unknown) => notify.scenarios.property.photoUpdateError(error));
  };

  const handleDeleteConfirm = (): void => {
    setDeleteConfirmOpen(false);
    void deletePhoto
      .mutateAsync({ id: propertyId })
      .catch((error: unknown) => notify.scenarios.property.photoDeleteError(error));
  };

  return (
    <div className="flex flex-col items-center gap-2">
      <PropertyAvatar surface="hero" type={type} photoUrl={displayUrl} />
      <input
        ref={fileInputRef}
        type="file"
        accept={PHOTO_INPUT_ACCEPT}
        className="hidden"
        onChange={handleFileChange}
      />
      {hasPhoto ? (
        <>
          <Button
            type="button"
            variant="clear"
            className="text-base"
            loading={uploadPhoto.isPending}
            onClick={openPicker}
          >
            Заменить фото
          </Button>
          <Button
            type="button"
            variant="clear"
            className="text-base"
            loading={deletePhoto.isPending}
            onClick={() => setDeleteConfirmOpen(true)}
          >
            Удалить фото
          </Button>
        </>
      ) : (
        <Button
          type="button"
          variant="clear"
          className="text-base"
          loading={uploadPhoto.isPending}
          onClick={openPicker}
        >
          Добавить фото
        </Button>
      )}
      <ConfirmDialog
        open={deleteConfirmOpen}
        onOpenChange={setDeleteConfirmOpen}
        title="Удалить фото?"
        description="Фото объекта будет удалено"
        confirmLabel="Удалить"
        confirmVariant="danger"
        pending={deletePhoto.isPending}
        onConfirm={handleDeleteConfirm}
      />
    </div>
  );
}
