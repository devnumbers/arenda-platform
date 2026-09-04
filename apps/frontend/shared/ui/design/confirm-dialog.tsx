'use client';

import type { JSX } from 'react';
import { Button } from './button';
import { Modal, ModalContent } from './modal';

/** Confirm-диалог дизайн-слоя — единственный канон подтверждений
 * (#505, замена легаси HeroUI ConfirmModal ADR 0050; унификация
 * 2026-09-04 поглотила design/confirm-modal). Контролируемый поверх
 * адаптивного Modal: карточка ≥768px, нижний шит с ручкой уже. Заголовок
 * обязателен (a11y: контент ссылается на Title), описание опционально.
 * Кнопки в ряд: отмена (secondary) + подтверждение (primary, для
 * разрушительных действий confirmVariant="danger"). Закрытие — на
 * потребителе: onConfirm вызывается, диалог закрывает потребитель
 * (обычно в onSuccess мутации, как в задачах) или сам через
 * onOpenChange(false). `pending` — подтверждение в полёте: кнопка
 * в loading, отмена/оверлей/свайп закрытие глушат. */
export type ConfirmDialogProps = {
  readonly open: boolean;
  readonly onOpenChange: (open: boolean) => void;
  readonly title: string;
  readonly description?: string;
  readonly confirmLabel: string;
  readonly cancelLabel?: string;
  readonly confirmVariant?: 'primary' | 'danger';
  /** Подтверждение в полёте: кнопка в loading, закрытие глушится. */
  readonly pending?: boolean;
  readonly onConfirm: () => void;
};

export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  confirmLabel,
  cancelLabel = 'Отмена',
  confirmVariant = 'primary',
  pending = false,
  onConfirm,
}: ConfirmDialogProps): JSX.Element {
  return (
    <Modal
      open={open}
      onOpenChange={(next) => {
        if (!pending) {
          onOpenChange(next);
        }
      }}
    >
      <ModalContent title={title} description={description}>
        <div className="grid grid-cols-2 gap-2">
          <Button variant="secondary" disabled={pending} onClick={() => onOpenChange(false)}>
            {cancelLabel}
          </Button>
          <Button variant={confirmVariant} loading={pending} onClick={onConfirm}>
            {confirmLabel}
          </Button>
        </div>
      </ModalContent>
    </Modal>
  );
}
