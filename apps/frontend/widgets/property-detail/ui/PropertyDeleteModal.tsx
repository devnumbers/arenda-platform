'use client';

import type { JSX } from 'react';
import { useState } from 'react';
import { Modal } from '@heroui/react';
import { Button } from '@/shared/ui/button';
import { pluralize } from '@/shared/lib/pluralize';
import type { components } from '@/shared/api/generated';
import type { DeletePropertyMode } from '@/features/properties/api/hooks';
import styles from './PropertyDeleteModal.module.css';

export type PropertyDeleteModalProps = {
    readonly isOpen: boolean;
    readonly onClose: () => void;
    readonly onDelete: (mode: DeletePropertyMode) => void;
    readonly onEndLease: () => void;
    readonly currentLease: components['schemas']['LeaseResponse'] | null | undefined;
    readonly membersCount?: number;
    readonly deletingMode?: DeletePropertyMode | null;
    readonly isCompletingLease?: boolean;
};

export function PropertyDeleteModal({
    isOpen,
    onClose,
    onDelete,
    onEndLease,
    currentLease,
    membersCount = 0,
    deletingMode = null,
    isCompletingLease = false,
}: PropertyDeleteModalProps): JSX.Element {
    const [cascadeConfirm, setCascadeConfirm] = useState(false);
    const [wasOpen, setWasOpen] = useState(isOpen);

    if (isOpen !== wasOpen) {
        setWasOpen(isOpen);
        if (!isOpen) {
            setCascadeConfirm(false);
        }
    }

    const handleOpenChange = (open: boolean): void => {
        if (!open) {
            onClose();
        }
    };

    const isBlocked = Boolean(currentLease);
    const heading = isBlocked
        ? 'Нельзя удалить объект'
        : cascadeConfirm
            ? 'Удалить все данные?'
            : 'Удалить объект?';

    return (
        <Modal isOpen={isOpen} onOpenChange={handleOpenChange}>
            <Modal.Backdrop>
                <Modal.Container placement="center" size="sm">
                    <Modal.Dialog aria-label={heading}>
                        <Modal.Header>
                            <Modal.Heading>{heading}</Modal.Heading>
                        </Modal.Header>
                        <Modal.Body>
                            <p>
                                {isBlocked
                                    ? 'Чтобы удалить объект, сначала завершите текущую аренду.'
                                    : cascadeConfirm
                                        ? 'Объект и все связанные данные — аренды, операции, напоминания и фотографии — будут удалены безвозвратно. Это действие нельзя отменить.'
                                        : 'Вы хотите удалить все данные, связанные с объектом (аренды, операции) или только сам объект?'}
                            </p>
                            {membersCount > 0 && (
                                <p className={styles.notice}>
                                    {`С объектом ${pluralize(membersCount, 'работает', 'работают', 'работают')} ${membersCount} ${pluralize(membersCount, 'участник', 'участника', 'участников')}.`}
                                </p>
                            )}
                        </Modal.Body>
                        <Modal.Footer>
                            <div className={styles.footer}>
                                {isBlocked ? (
                                    <>
                                        <Button variant="secondary" onClick={onClose} type="button">
                                            Отмена
                                        </Button>
                                        <Button
                                            variant="primary"
                                            onClick={onEndLease}
                                            loading={isCompletingLease}
                                            type="button"
                                        >
                                            Завершить аренду
                                        </Button>
                                    </>
                                ) : cascadeConfirm ? (
                                    <>
                                        <Button
                                            variant="secondary"
                                            onClick={() => setCascadeConfirm(false)}
                                            type="button"
                                        >
                                            Назад
                                        </Button>
                                        <Button
                                            variant="primary"
                                            onClick={() => onDelete('cascade')}
                                            loading={deletingMode === 'cascade'}
                                            type="button"
                                        >
                                            Удалить всё
                                        </Button>
                                    </>
                                ) : (
                                    <>
                                        <Button variant="secondary" onClick={onClose} type="button">
                                            Отмена
                                        </Button>
                                        <Button
                                            variant="primary"
                                            onClick={() => onDelete('detach')}
                                            loading={deletingMode === 'detach'}
                                            disabled={deletingMode === 'cascade'}
                                            type="button"
                                        >
                                            Только объект
                                        </Button>
                                        <Button
                                            variant="primary"
                                            onClick={() => setCascadeConfirm(true)}
                                            type="button"
                                        >
                                            Все данные
                                        </Button>
                                    </>
                                )}
                            </div>
                        </Modal.Footer>
                    </Modal.Dialog>
                </Modal.Container>
            </Modal.Backdrop>
        </Modal>
    );
}
