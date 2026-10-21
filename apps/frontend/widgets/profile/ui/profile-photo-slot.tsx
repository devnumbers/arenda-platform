'use client';

import { useEffect, useRef, useState, type ChangeEvent, type JSX } from 'react';
import { notify } from '@/shared/lib/notifications';
import {
  PHOTO_FILE_TOO_LARGE_MESSAGE,
  photoFileTooLarge,
} from '@/shared/lib/photo';
import { type MePhotoStage, mePhotoDisplay } from '@/features/profile';
import { Button, ConfirmDialog, PhotoProgressRing, PhotoRemoveBadge } from '@/shared/ui/design';
import { ProfileAvatar } from './profile-avatar';

/**
 * Слот фото профиля на экране аккаунта (тикет #1230; макет 1789:99036,
 * канон правки 1299:51572/73; ADR 0065): круг-плейсхолдер с BoldUser либо
 * фото. Экран — автосейв-форма, изменения применяются сразу (решение
 * владельца: «при изменении фото оно сразу должно меняться»): пикер
 * запускает мутацию немедленно, stage держит data-URL превью; успех гасит
 * stage, провал ДЕРЖИТ файл (план 037) — тап по кругу повторяет ту же
 * загрузку, «Повторить» в тосте — то же действие, ссылка «Обновить фото»
 * остаётся пикером (замена файла). Круг — настоящая кнопка (aria-label,
 * клавиатура): без фото открывает пикер, с фото — замену, с провалом —
 * повтор. Бейдж-корзина — сосед круга в DOM, не потомок: клик по нему
 * открывает ConfirmDialog (однотапный бейдж не стирает молча),
 * подтверждение — сразу DELETE: диалог держит pending, пока мутация
 * летит, и закрывается по её факту (канон ConfirmDialog.pending). На
 * время мутаций (busy) круг и бейдж глушатся, ссылка прячется с
 * сохранением слота — ноль layout shift; бейдж гаснет по канону
 * дизейблов.
 */

const PHOTO_INPUT_ACCEPT = 'image/jpeg,image/png,image/webp';

/** Кнопка-круг (клик по фото открывает пикер): фокус-ринг по канону
 * IconButton, в полёте глушится без затемнения — круг «как был». Отклик —
 * канон рядов: пресс-скейл + приглушение на наведении (работает и с фото,
 * и с пустым кругом, затемнения фона под фото не видно). */
const PHOTO_BUTTON_CLASS =
  'pressable flex cursor-pointer rounded-full outline-none hover:opacity-80 focus-visible:ring-2 focus-visible:ring-primary disabled:pointer-events-none';

export type ProfilePhotoSlotProps = {
  /** same-origin стриминговый путь выдачи (ADR 0065), null — фото нет. */
  readonly photoUrl?: string | null;
  /** Загрузка в круге: превью выбранного файла; failed — прошлый провал
   * (тап по кругу = повтор, см. onRetry). */
  readonly stage: MePhotoStage | null;
  /** Фото-мутация в полёте — управление слота глушится, диалог держит
   * pending. */
  readonly busy: boolean;
  /** Проценты выгрузки 0–100 (план 036); null — загрузки нет, кольца нет. */
  readonly progress?: number | null;
  /** Файл выбран (кап уже проверен) — экран запускает загрузку сразу. */
  readonly onFileChosen: (file: File) => void;
  /** Удаление подтверждено — экран сразу вызывает DELETE. */
  readonly onRemove: () => void;
  /** Повтор провалившейся загрузки тем же файлом (план 037). */
  readonly onRetry: () => void;
};

export function ProfilePhotoSlot({
  photoUrl,
  stage,
  busy,
  progress = null,
  onFileChosen,
  onRemove,
  onRetry,
}: ProfilePhotoSlotProps): JSX.Element {
  const [removeConfirmOpen, setRemoveConfirmOpen] = useState(false);
  const busyRef = useRef(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Диалог удаления живёт на мутации: pending глушит закрытие на полёте,
  // закрытие — по спаду busy (факт успеха или отказа), канон
  // ConfirmDialog.pending («диалог закрывает потребитель»).
  useEffect(() => {
    if (busyRef.current && !busy && removeConfirmOpen) {
      setRemoveConfirmOpen(false);
    }
    busyRef.current = busy;
  }, [busy, removeConfirmOpen]);

  const displayUrl = mePhotoDisplay(photoUrl ?? null, stage);
  const hasPhoto = displayUrl !== null;
  // Провал прошлой попытки (план 037): круг зовёт повтор, не пикер.
  const retrying = stage?.failed === true;

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
    onFileChosen(file);
  };

  const avatarButton = (
    <button
      type="button"
      aria-label={
        retrying ? 'Повторить загрузку фото' : hasPhoto ? 'Обновить фото' : 'Добавить фото'
      }
      disabled={busy}
      className={PHOTO_BUTTON_CLASS}
      onClick={retrying ? onRetry : openPicker}
    >
      <ProfileAvatar photoUrl={displayUrl} />
    </button>
  );

  return (
    <div className="flex flex-col items-center gap-2">
      <div className="relative">
        {avatarButton}
        {progress !== null && <PhotoProgressRing percent={progress} />}
        {hasPhoto && (
          <PhotoRemoveBadge disabled={busy} onClick={() => setRemoveConfirmOpen(true)} />
        )}
      </div>
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
        confirmLabel="Удалить"
        confirmVariant="danger"
        pending={busy}
        onConfirm={onRemove}
      />
    </div>
  );
}
