'use client';

import type { JSX } from 'react';
import { Modal } from '@heroui/react';
import { Button } from '@/shared/ui/button';
import { pluralize } from '@/shared/lib/pluralize';
import styles from './PropertyArchiveModal.module.css';

export type PropertyArchiveModalProps = {
    readonly isOpen: boolean;
    readonly onClose: () => void;
    readonly onArchive: () => void;
    readonly membersCount?: number;
    readonly isArchiving?: boolean;
};

export function PropertyArchiveModal({
    isOpen,
    onClose,
    onArchive,
    membersCount = 0,
    isArchiving = false,
}: PropertyArchiveModalProps): JSX.Element {
    const handleOpenChange = (open: boolean): void => {
        if (!open) {
            onClose();
        }
    };

    return (
        <Modal isOpen={isOpen} onOpenChange={handleOpenChange}>
            <Modal.Backdrop>
                <Modal.Container placement="center" size="sm">
                    <Modal.Dialog aria-label="Архивировать объект?">
                        <Modal.Header>
                            <Modal.Heading>Архивировать объект?</Modal.Heading>
                        </Modal.Header>
                        <Modal.Body>
                            <p>
                                Объект будет перенесён в архив. Вы сможете вернуть его в работу в любой момент.
                            </p>
                            {membersCount > 0 && (
                                <p className={styles.notice}>
                                    {`С объектом ${pluralize(membersCount, 'работает', 'работают', 'работают')} ${membersCount} ${pluralize(membersCount, 'участник', 'участника', 'участников')}. После архивации они увидят объект в режиме просмотра.`}
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
                                    onClick={onArchive}
                                    loading={isArchiving}
                                    type="button"
                                >
                                    Архивировать
                                </Button>
                            </div>
                        </Modal.Footer>
                    </Modal.Dialog>
                </Modal.Container>
            </Modal.Backdrop>
        </Modal>
    );
}
