'use client';

import type { JSX } from 'react';
import { Button } from './button';
import { Modal, ModalContent } from './modal';

/** Диалог подтверждения дизайн-слоя (карта #497, Figma 1539:77671 — шит
 * «Удалить все выполненные задачи?»): заголовок H3, серый подзаголовок
 * R/400 16 и пара кнопок в ряд — «Отменить» (secondary) и дейнджер-
 * подтверждение (danger). На мобайле — нижний шит с ручкой, на десктопе —
 * карточка (общий Modal-шелл). Пока подтверждение в полёте, кнопка
 * «Удалить» показывает загрузку, закрытие глушится. */
export type ConfirmModalProps = {
  readonly open: boolean;
  readonly onOpenChange: (open: boolean) => void;
  readonly title: string;
  readonly description: string;
  readonly cancelLabel: string;
  readonly confirmLabel: string;
  readonly onConfirm: () => void;
  /** Процесс выполнения подтверждения: кнопка в loading, закрытие глушится. */
  readonly pending?: boolean;
};

export function ConfirmModal({
  open,
  onOpenChange,
  title,
  description,
  cancelLabel,
  confirmLabel,
  onConfirm,
  pending = false,
}: ConfirmModalProps): JSX.Element {
  return (
    <Modal
      open={open}
      onOpenChange={(next) => {
        if (!pending) {
          onOpenChange(next);
        }
      }}
    >
      <ModalContent
        title={title}
        description={description}
        descriptionClassName="text-base leading-[18px]"
      >
        <div className="flex items-center gap-2">
          <Button
            variant="secondary"
            className="flex-1"
            disabled={pending}
            onClick={() => onOpenChange(false)}
          >
            {cancelLabel}
          </Button>
          <Button variant="danger" className="flex-1" loading={pending} onClick={onConfirm}>
            {confirmLabel}
          </Button>
        </div>
      </ModalContent>
    </Modal>
  );
}
