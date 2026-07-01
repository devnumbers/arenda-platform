'use client';

import type { JSX } from 'react';
import { Modal } from '@heroui/react';
import { Button } from '@/shared/ui/button';

export type PropertyBlockedModalProps = {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onContinue: () => void;
};

export function PropertyBlockedModal({
  isOpen,
  onClose,
  onContinue,
}: PropertyBlockedModalProps): JSX.Element {
  const handleOpenChange = (open: boolean): void => {
    if (!open) {
      onClose();
    }
  };

  const handleContinue = (): void => {
    onContinue();
    onClose();
  };

  return (
    <Modal isOpen={isOpen} onOpenChange={handleOpenChange}>
      <Modal.Backdrop>
        <Modal.Container placement="center" size="sm">
          <Modal.Dialog aria-label="Нельзя изменить статус">
            <Modal.Header>
              <Modal.Heading>Нельзя изменить статус</Modal.Heading>
            </Modal.Header>
            <Modal.Body>
              <p>Статус нельзя изменить, пока есть незавершённая аренда.</p>
            </Modal.Body>
            <Modal.Footer>
              <Button variant="secondary" onClick={onClose} type="button">
                Отменить
              </Button>
              <Button variant="primary" onClick={handleContinue} type="button">
                Продолжить
              </Button>
            </Modal.Footer>
          </Modal.Dialog>
        </Modal.Container>
      </Modal.Backdrop>
    </Modal>
  );
}
