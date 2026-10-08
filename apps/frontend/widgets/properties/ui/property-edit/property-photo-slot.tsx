'use client';

import { useRef, useState, type ChangeEvent, type JSX } from 'react';
import { notify } from '@/shared/lib/notifications';
import {
  PHOTO_FILE_TOO_LARGE_MESSAGE,
  type PropertyPhotoStage,
  photoFileTooLarge,
  propertyPhotoDisplay,
  usePropertyPhotoBuster,
} from '@/features/properties';
import { PropertyAvatar, type PropertyType } from '@/entities/property';
import { Button, circleIconRing, ConfirmDialog, PhotoRemoveBadge } from '@/shared/ui/design';

/**
 * Слот фото объекта на правке (тикет #1228; ADR 0065): круг-плейсхолдер
 * по типу (поверхность hero канона PropertyAvatar) либо фото. Управляемый
 * и без сети: изменение фото живёт в черновике формы (stage,
 * PropertyPhotoStage — решение владельца 08.10), что показывать, решает
 * propertyPhotoDisplay (staged-превью → серверное фото с бастером →
 * глиф), а применяют изменение кнопки сохранения экрана — onFileChosen и
 * onRemove только переставляют stage, увлечение без сохранения сервер не
 * трогает. Круг — настоящая кнопка (aria-label, клавиатура; решение
 * владельца 08.10): без фото открывает пикер, с фото — замену. Бейдж-
 * корзина — сосед круга в DOM, не потомок: клик по нему открывает
 * ConfirmDialog («удаление применится при сохранении»), а не пикер.
 * На время фото-мутаций (busy) круг и бейдж глушатся, ссылка прячется с
 * сохранением слота — ноль layout shift; бейдж гаснет по канону
 * дизейблов.
 */

const PHOTO_INPUT_ACCEPT = 'image/jpeg,image/png,image/webp';

/** Кнопка-круг (клик по фото открывает пикер): фокус-ринг по канону
 * IconButton, в полёте глушится без затемнения — круг «как был». */
const PHOTO_BUTTON_CLASS =
  'flex cursor-pointer rounded-full outline-none focus-visible:ring-2 focus-visible:ring-primary disabled:pointer-events-none';

export type PropertyPhotoSlotProps = {
  readonly propertyId: string;
  /** same-origin стриминговый путь выдачи (ADR 0065), null — фото нет. */
  readonly photoUrl?: string | null;
  /** Незавершённое изменение фото из черновика экрана (см. выше). */
  readonly stage: PropertyPhotoStage | null;
  /** Фото-мутация сохранения в полёте — управление слота глушится. */
  readonly busy: boolean;
  /** Тип объекта — выбирает глиф-плейсхолдер, следует за черновиком формы. */
  readonly type?: PropertyType;
  /** Файл выбран (кап уже проверен) — экран кладёт его в stage. */
  readonly onFileChosen: (file: File) => void;
  /** Удаление подтверждено — экран ставит stage remove. */
  readonly onRemove: () => void;
};

export function PropertyPhotoSlot({
  propertyId,
  photoUrl,
  stage,
  busy,
  type,
  onFileChosen,
  onRemove,
}: PropertyPhotoSlotProps): JSX.Element {
  const photoBuster = usePropertyPhotoBuster(propertyId);
  const [removeConfirmOpen, setRemoveConfirmOpen] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const displayUrl = propertyPhotoDisplay(photoUrl ?? null, photoBuster, stage);
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
    // решает бэкенд при сохранении, его problem+json текст виден в тосте
    // отказа.
    if (photoFileTooLarge(file.size)) {
      notify.error(PHOTO_FILE_TOO_LARGE_MESSAGE);
      return;
    }
    onFileChosen(file);
  };

  const avatarButton = (
    <button
      type="button"
      aria-label={hasPhoto ? 'Обновить фото' : 'Добавить фото'}
      disabled={busy}
      className={PHOTO_BUTTON_CLASS}
      onClick={openPicker}
    >
      <PropertyAvatar
        surface="hero"
        type={type}
        photoUrl={displayUrl}
        className={hasPhoto ? circleIconRing.white : undefined}
      />
    </button>
  );

  return (
    <div className="flex flex-col items-center gap-2">
      {hasPhoto ? (
        <div className="relative">
          {avatarButton}
          <PhotoRemoveBadge disabled={busy} onClick={() => setRemoveConfirmOpen(true)} />
        </div>
      ) : (
        avatarButton
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
        open={removeConfirmOpen}
        onOpenChange={setRemoveConfirmOpen}
        title="Удалить фото?"
        description="Фото объекта будет удалено при сохранении"
        confirmLabel="Удалить"
        confirmVariant="danger"
        onConfirm={() => {
          setRemoveConfirmOpen(false);
          onRemove();
        }}
      />
    </div>
  );
}
