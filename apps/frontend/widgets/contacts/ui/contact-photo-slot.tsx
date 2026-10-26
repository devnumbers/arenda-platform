'use client';

import { useRef, useState, type ChangeEvent, type JSX } from 'react';
import { notify } from '@/shared/lib/notifications';
import { BoldUser } from '@/shared/assets/icons';
import { PHOTO_FILE_TOO_LARGE_MESSAGE, photoFileTooLarge } from '@/shared/lib/photo';
import { type ContactPhotoStage, contactPhotoDisplay } from '@/features/contacts';
import { Button, circleIconRing, CirclePhoto, ConfirmDialog, PhotoProgressRing, PhotoRemoveBadge } from '@/shared/ui/design';

/**
 * Слот фото контакта в форме (тикет #1229; макеты 1281:48439 /
 * 1302:58933, канон правки 1299:51572/73; ADR 0065): круг-плейсхолдер с
 * BoldUser либо фото. Управляемый и без сети: изменение фото живёт в
 * черновике формы (stage, ContactPhotoStage — решения владельца 08.10 с
 * #1228), что показывать, решает contactPhotoDisplay (staged-превью →
 * серверное фото, URL версионирован бэком → глиф), а применяют изменение кнопки
 * сохранения экрана — onFileChosen и onRemove только переставляют stage,
 * уход без сохранения сервер не трогает. Круг — настоящая кнопка
 * (aria-label, клавиатура): без фото открывает пикер, с фото — замену.
 * Бейдж-корзина — сосед круга в DOM, не потомок: клик по нему открывает
 * ConfirmDialog, а не пикер. На время фото-мутаций (busy) круг и бейдж
 * глушатся, ссылка прячется с сохранением слота — ноль layout shift;
 * бейдж гаснет по канону дизейблов.
 */

const PHOTO_INPUT_ACCEPT = 'image/jpeg,image/png,image/webp';

/** Кнопка-круг (клик по фото открывает пикер): фокус-ринг по канону
 * IconButton, в полёте глушится без затемнения — круг «как был». Отклик —
 * канон рядов: пресс-скейл + приглушение на наведении (работает и с фото,
 * и с пустым кругом, затемнения фона под фото не видно). */
const PHOTO_BUTTON_CLASS =
  'pressable flex cursor-pointer rounded-full outline-none hover:opacity-80 focus-visible:ring-2 focus-visible:ring-primary disabled:pointer-events-none';

export type ContactPhotoSlotProps = {
  /** same-origin стриминговый путь выдачи (ADR 0065), null/undefined — фото нет. */
  readonly photoUrl?: string | null;
  /** Blur-кадр серверного фото (ADR 0065 аменд, решение #1594); при
   * незавершённом стейдже не рисуется — в круге staged-превью, байты
   * уже на руках. */
  readonly photoBlur?: string | null;
  /** Незавершённое изменение фото из черновика экрана (см. выше). */
  readonly stage: ContactPhotoStage | null;
  /** Фото-мутация сохранения в полёте — управление слота глушится. */
  readonly busy: boolean;
  /** Проценты выгрузки 0–100 (план 036); null — загрузки нет, кольца нет. */
  readonly progress?: number | null;
  /** Файл выбран (кап уже проверен) — экран кладёт его в stage. */
  readonly onFileChosen: (file: File) => void;
  /** Удаление подтверждено — экран решает stage по наличию серверного фото. */
  readonly onRemove: () => void;
};

export function ContactPhotoSlot({
  photoUrl,
  photoBlur,
  stage,
  busy,
  progress = null,
  onFileChosen,
  onRemove,
}: ContactPhotoSlotProps): JSX.Element {
  // Механика показа (битое фото → заглушка, гард '', ожидание) — примитив
  // CirclePhoto (#1423); при битом фото круг с кантом остаётся — канон
  // UserAvatar/профильного слота.
  const [removeConfirmOpen, setRemoveConfirmOpen] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const displayUrl = contactPhotoDisplay(photoUrl ?? null, stage);
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

  const photoCircleButton = (
    <button
      type="button"
      aria-label={hasPhoto ? 'Обновить фото' : 'Добавить фото'}
      disabled={busy}
      className={PHOTO_BUTTON_CLASS}
      onClick={openPicker}
    >
      <span
        className={[
          'flex h-24 w-24 items-center justify-center rounded-full bg-surface-muted',
          hasPhoto ? `overflow-hidden ${circleIconRing.white}` : undefined,
        ]
          .filter(Boolean)
          .join(' ')}
      >
        <CirclePhoto
          photoUrl={displayUrl}
          photoBlur={stage === null ? photoBlur : null}
          glyph={<BoldUser className="h-13 w-13 text-content-tertiary" />}
        />
      </span>
    </button>
  );

  return (
    <div className="flex flex-col items-center gap-2">
      <div className="relative">
        {photoCircleButton}
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
        description={
          photoUrl
            ? 'Фото контакта будет удалено при сохранении'
            : 'Выбранное фото не сохранится'
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
