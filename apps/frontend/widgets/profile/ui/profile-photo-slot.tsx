'use client';

import { useRef, useState, type ChangeEvent, type JSX } from 'react';
import { notify } from '@/shared/lib/notifications';
import {
  PHOTO_FILE_TOO_LARGE_MESSAGE,
  photoFileTooLarge,
} from '@/shared/lib/photo';
import { type MePhotoStage, mePhotoDisplay, useMePhotoBuster } from '@/features/profile';
import { Button, ConfirmDialog, PhotoRemoveBadge } from '@/shared/ui/design';
import { ProfileAvatar } from './profile-avatar';

/**
 * Слот фото профиля на экране аккаунта (тикет #1230; макет 1789:99036,
 * канон правки 1299:51572/73; ADR 0065): круг-плейсхолдер с BoldUser либо
 * фото. Управляемый и без сети: изменение фото живёт в черновике экрана
 * (stage, MePhotoStage — решения владельца 08.10 с #1228), что показывать,
 * решает mePhotoDisplay (staged-превью → серверное фото с бастером →
 * глиф), а применяет изменение коммит формы аккаунта — onFileChosen и
 * onRemove только переставляют stage, уход без коммита сервер не трогает.
 * Круг — настоящая кнопка (aria-label, клавиатура): без фото открывает
 * пикер, с фото — замену. Бейдж-корзина — сосед круга в DOM, не потомок:
 * клик по нему открывает ConfirmDialog, а не пикер. На время фото-мутаций
 * (busy) круг и бейдж глушатся, ссылка прячется с сохранением слота —
 * ноль layout shift; бейдж гаснет по канону дизейблов.
 */

const PHOTO_INPUT_ACCEPT = 'image/jpeg,image/png,image/webp';

/** Кнопка-круг (клик по фото открывает пикер): фокус-ринг по канону
 * IconButton, в полёте глушится без затемнения — круг «как был». */
const PHOTO_BUTTON_CLASS =
  'flex cursor-pointer rounded-full outline-none focus-visible:ring-2 focus-visible:ring-primary disabled:pointer-events-none';

export type ProfilePhotoSlotProps = {
  /** same-origin стриминговый путь выдачи (ADR 0065), null — фото нет. */
  readonly photoUrl?: string | null;
  /** Незавершённое изменение фото из черновика экрана (см. выше). */
  readonly stage: MePhotoStage | null;
  /** Фото-мутация коммита в полёте — управление слота глушится. */
  readonly busy: boolean;
  /** Файл выбран (кап уже проверен) — экран кладёт его в stage. */
  readonly onFileChosen: (file: File) => void;
  /** Удаление подтверждено — экран решает stage по наличию серверного фото. */
  readonly onRemove: () => void;
};

export function ProfilePhotoSlot({
  photoUrl,
  stage,
  busy,
  onFileChosen,
  onRemove,
}: ProfilePhotoSlotProps): JSX.Element {
  const photoBuster = useMePhotoBuster();
  const [removeConfirmOpen, setRemoveConfirmOpen] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const displayUrl = mePhotoDisplay(photoUrl ?? null, photoBuster, stage);
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
      <ProfileAvatar photoUrl={displayUrl} />
    </button>
  );

  return (
    <div className="flex flex-col items-center gap-2">
      {hasPhoto ? (
        // Выемку правого-верхнего края держит пара: белое кольцо 2.5px на
        // круге (CircleIcon white) и белая подложка бейджа (канон
        // 1299:51572/73).
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
        description={
          photoUrl ? 'Фото профиля будет удалено при сохранении' : 'Выбранное фото не сохранится'
        }
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
