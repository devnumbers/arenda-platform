'use client';

import type { JSX } from 'react';
import { Modal } from '@heroui/react';
import { Button } from '@/shared/ui/button';

export type PropertyEndLeaseModalProps = {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onConfirm: () => void;
};

export function PropertyEndLeaseModal({
  isOpen,
  onClose,
  onConfirm,
}: PropertyEndLeaseModalProps): JSX.Element {
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
    <Modal>
      <Modal.Backdrop isOpen={isOpen} onOpenChange={handleOpenChange}>
        <Modal.Container placement="center" size="sm">
          <Modal.Dialog aria-label="Завершить аренду?">
            <Modal.Header>
              <Modal.Heading>Завершить аренду?</Modal.Heading>
            </Modal.Header>
            <Modal.Body>
              <p>
                Информацию по этой аренде можно будет посмотреть в разделе
                «Аренда».
              </p>
            </Modal.Body>
            <Modal.Footer>
              <Button variant="secondary" onClick={onClose} type="button">
                Отменить
              </Button>
              <Button variant="primary" onClick={handleConfirm} type="button">
                Завершить аренду
              </Button>
            </Modal.Footer>
          </Modal.Dialog>
        </Modal.Container>
      </Modal.Backdrop>
    </Modal>
  );
}
