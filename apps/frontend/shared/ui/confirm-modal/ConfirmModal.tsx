'use client';

import type { JSX } from 'react';
import { Modal } from '@heroui/react';
import { Button } from '@/shared/ui/button';

export type ConfirmModalProps = {
  readonly isOpen: boolean;
  readonly title: string;
  readonly description?: string;
  readonly confirmLabel: string;
  readonly cancelLabel?: string;
  readonly onClose: () => void;
  readonly onConfirm: () => void;
};

export function ConfirmModal({
  isOpen,
  title,
  description,
  confirmLabel,
  cancelLabel = 'Отменить',
  onClose,
  onConfirm,
}: ConfirmModalProps): JSX.Element {
  const handleOpenChange = (open: boolean): void => {
    if (!open) {
      onClose();
    }
  };

  const handleConfirm = (): void => {
    onConfirm();
    onClose();
  };

  return (
    <Modal isOpen={isOpen} onOpenChange={handleOpenChange}>
      <Modal.Backdrop>
        <Modal.Container placement="center" size="sm">
          <Modal.Dialog aria-label={title}>
            <Modal.Header>
              <Modal.Heading>{title}</Modal.Heading>
            </Modal.Header>
            {description && (
              <Modal.Body>
                <p>{description}</p>
              </Modal.Body>
            )}
            <Modal.Footer>
              <Button variant="secondary" onClick={onClose} type="button">
                {cancelLabel}
              </Button>
              <Button variant="primary" onClick={handleConfirm} type="button">
                {confirmLabel}
              </Button>
            </Modal.Footer>
          </Modal.Dialog>
        </Modal.Container>
      </Modal.Backdrop>
    </Modal>
  );
}
