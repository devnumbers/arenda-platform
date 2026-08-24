'use client';

import type { JSX } from 'react';
import { Modal } from '@heroui/react';
import { Button } from '@/shared/ui/button';
import { pluralize } from '@/shared/lib/pluralize';
import type { DeletePropertyMode } from '@/features/properties';
import styles from './PropertyDeleteModal.module.css';

export type PropertyDeleteModalProps = {
    readonly isOpen: boolean;
    readonly onClose: () => void;
    readonly onDelete: (mode: DeletePropertyMode) => void;
    readonly membersCount?: number;
    readonly deletingMode?: DeletePropertyMode | null;
};

export function PropertyDeleteModal({
    isOpen,
    onClose,
    onDelete,
    membersCount = 0,
    deletingMode = null,
}: PropertyDeleteModalProps): JSX.Element {
    const handleOpenChange = (open: boolean): void => {
        if (!open) {
            onClose();
        }
    };

    return (
        <Modal isOpen={isOpen} onOpenChange={handleOpenChange}>
            <Modal.Backdrop>
                <Modal.Container placement="center" size="sm">
                    <Modal.Dialog aria-label="Удалить объект?">
                        <Modal.Header>
                            <Modal.Heading>Удалить объект?</Modal.Heading>
                        </Modal.Header>
                        <Modal.Body>
                            <p>Объект и его фотографии будут удалены безвозвратно. Это действие нельзя отменить.</p>
                            {membersCount > 0 && (
                                <p className={styles.notice}>
                                    {`С объектом ${pluralize(membersCount, 'работает', 'работают', 'работают')} ${membersCount} ${pluralize(membersCount, 'участник', 'участника', 'участников')}.`}
                                </p>
                            )}
                        </Modal.Body>
                        <Modal.Footer>
                            <div className={styles.footer}>
                                <Button variant="secondary" onClick={onClose} type="button">
                                    Отмена
                                </Button>
                                <Button
                                    variant="primary"
                                    onClick={() => onDelete('cascade')}
                                    loading={deletingMode === 'cascade'}
                                    type="button"
                                >
                                    Удалить
                                </Button>
                            </div>
                        </Modal.Footer>
                    </Modal.Dialog>
                </Modal.Container>
            </Modal.Backdrop>
        </Modal>
    );
}
