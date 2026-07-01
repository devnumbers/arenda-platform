'use client';

import type { JSX } from 'react';
import { Modal } from '@heroui/react';
import { Button } from '@/shared/ui/button';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import styles from './PropertyDepositReturnModal.module.css';

export type PropertyDepositReturnModalProps = {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onConfirm: () => void;
  readonly depositAmountKopecks: number;
};

export function PropertyDepositReturnModal({
  isOpen,
  onClose,
  onConfirm,
  depositAmountKopecks,
}: PropertyDepositReturnModalProps): JSX.Element {
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
          <Modal.Dialog aria-label="Возврат залога">
            <Modal.Header>
              <Modal.Heading>Возврат залога</Modal.Heading>
            </Modal.Header>
            <Modal.Body>
              <p className={styles.amountLabel}>Сумма возврата</p>
              <p className={styles.amountValue}>
                {formatMoneyKopecks(depositAmountKopecks)}
              </p>
            </Modal.Body>
            <Modal.Footer>
              <Button variant="secondary" onClick={onClose} type="button">
                Отменить
              </Button>
              <Button variant="primary" onClick={handleConfirm} type="button">
                Продолжить
              </Button>
            </Modal.Footer>
          </Modal.Dialog>
        </Modal.Container>
      </Modal.Backdrop>
    </Modal>
  );
}
