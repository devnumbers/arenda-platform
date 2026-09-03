'use client';

import type { JSX } from 'react';
import { Button } from './button';
import { Modal, ModalClose, ModalContent } from './modal';

/** Confirm-диалог дизайн-слоя (#505) — замена легаси HeroUI ConfirmModal
 * (shared/ui/confirm-modal, ADR 0050) на новых экранах. Контролируемый
 * поверх адаптивного Modal (карточка ≥768px / нижний шит уже); семантика
 * легаси сохранена: onConfirm вызывается и диалог закрывается сам, отмена
 * и закрытие оверлеем/свайпом — просто onOpenChange(false). Кнопки в ряд:
 * отмена secondary + подтверждение primary; разрушительное действие
 * (удаление контакта) — confirmVariant="danger". Заголовок обязателен
 * (a11y: контент ссылается на Title), описание — DialogDescription. */
export type ConfirmDialogProps = {
  readonly open?: boolean;
  readonly defaultOpen?: boolean;
  readonly onOpenChange?: (open: boolean) => void;
  readonly title: string;
  readonly description?: string;
  readonly confirmLabel: string;
  readonly cancelLabel?: string;
  readonly confirmVariant?: 'primary' | 'danger';
  readonly onConfirm: () => void;
};

export function ConfirmDialog({
  open,
  defaultOpen,
  onOpenChange,
  title,
  description,
  confirmLabel,
  cancelLabel = 'Отмена',
  confirmVariant = 'primary',
  onConfirm,
}: ConfirmDialogProps): JSX.Element {
  return (
    <Modal open={open} defaultOpen={defaultOpen} onOpenChange={onOpenChange}>
      <ModalContent title={title} description={description}>
        <div className="grid grid-cols-2 gap-2">
          <ModalClose asChild>
            <Button variant="secondary">{cancelLabel}</Button>
          </ModalClose>
          <ModalClose asChild>
            <Button variant={confirmVariant} onClick={onConfirm}>
              {confirmLabel}
            </Button>
          </ModalClose>
        </div>
      </ModalContent>
    </Modal>
  );
}
