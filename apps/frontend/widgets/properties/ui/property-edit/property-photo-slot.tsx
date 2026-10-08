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
import { Button, circleIconRing, ConfirmDialog, PhotoRemoveBadge } from '@/shared/ui/design';

/**
 * Слот фото объекта на правке (тикет #1228; ADR 0065): круг-плейсхолдер
 * по типу (поверхность hero канона PropertyAvatar) либо загруженное фото.
 * Канон правки/удаления — макет 1299:51572/73 (правка фото контакта,
 * эталон карты #1217): бейдж-корзина в выемке правого-верхнего края круга
 * (PhotoRemoveBadge; круг под ним — с кольцом цвета подложки
 * circleIconRing) и одна ссылка «Обновить фото» под кругом — канон Clear
 * (M/500 14, Figma 1134:55051); без фото бейджа нет, ссылка — «Добавить
 * фото». Удаление — через ConfirmDialog: однотапный бейдж не должен
 * молча стирать фото (решение владельца 08.10). На время запроса круг
 * остаётся ровно тем же, что был (прежнее фото либо глиф), ссылка
 * исчезает целиком, сохраняя свой слот — без полупрозрачного призрака
 * надписи и без сдвига формы (решение владельца 08.10; скрытие с
 * сохранением места = ноль layout shift); бейдж остаётся на месте и
 * гаснет по канону дизейблов — тонкий сигнал полёта. Фото живёт мимо
 * черновика формы — загрузка/удаление применяются сразу отдельными
 * эндпоинтами (`/properties/{id}/photo`, multipart POST и DELETE),
 * кнопки «Сохранить» не касаются. Ошибки — mutateAsync + catch с тостом
 * (канон форм; пер-колбэки mutate не используются).
 *
 * Байты выдачи кэшируются браузером (private, max-age=300) при неизменном
 * пути — URL для <img> собирается с бастером `?v=N` (photoDisplayUrl);
 * счётчик N живёт в react-query кэше (usePropertyPhotoBuster, пишут
 * мутации), поэтому переживает перемонтирования экрана.
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
  // В полёте глушим оба управления: одновременные загрузка и удаление
  // гоняются за один photo_key.
  const busy = uploadPhoto.isPending || deletePhoto.isPending;

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
      {hasPhoto ? (
        <div className="relative">
          <PropertyAvatar
            surface="hero"
            type={type}
            photoUrl={displayUrl}
            className={circleIconRing.white}
          />
          <PhotoRemoveBadge disabled={busy} onClick={() => setDeleteConfirmOpen(true)} />
        </div>
      ) : (
        <PropertyAvatar surface="hero" type={type} />
      )}
      <input
        ref={fileInputRef}
        type="file"
        accept={PHOTO_INPUT_ACCEPT}
        className="hidden"
        onChange={handleFileChange}
      />
      <Button
        type="button"
        variant="clear"
        disabled={busy}
        className={busy ? 'invisible' : undefined}
        onClick={openPicker}
      >
        {hasPhoto ? 'Обновить фото' : 'Добавить фото'}
      </Button>
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
