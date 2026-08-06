'use client';

import {useState, type FormEvent, type JSX} from 'react';
import {Modal} from '@heroui/react';
import {Button} from '@/shared/ui/button';
import {TextField} from '@/shared/ui/text-field';
import {Select} from '@/shared/ui/select';
import {memberRoleOptions, type MemberRole} from '@/features/access/lib/roles';
import styles from './PropertyInviteModal.module.css';

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export type PropertyInviteModalProps = {
    readonly isOpen: boolean;
    readonly isSubmitting: boolean;
    readonly onClose: () => void;
    readonly onInvite: (email: string, role: MemberRole) => Promise<boolean>;
};

export function PropertyInviteModal({
    isOpen,
    isSubmitting,
    onClose,
    onInvite,
}: PropertyInviteModalProps): JSX.Element {
    const [email, setEmail] = useState('');
    const [role, setRole] = useState<MemberRole>('full_access');
    const [isSubmitAttempted, setIsSubmitAttempted] = useState(false);
    const [wasOpen, setWasOpen] = useState(isOpen);

    if (isOpen !== wasOpen) {
        setWasOpen(isOpen);
        if (!isOpen) {
            setEmail('');
            setRole('full_access');
            setIsSubmitAttempted(false);
        }
    }

    const handleOpenChange = (open: boolean): void => {
        if (!open) {
            onClose();
        }
    };

    const trimmedEmail = email.trim();
    const isEmailValid = EMAIL_PATTERN.test(trimmedEmail);
    const canSubmit = isEmailValid && !isSubmitting;
    const emailError = isSubmitAttempted && !isEmailValid
        ? 'Введите корректный email'
        : undefined;

    const handleSubmit = async (event: FormEvent<HTMLFormElement>): Promise<void> => {
        event.preventDefault();
        setIsSubmitAttempted(true);
        if (!canSubmit) return;
        const succeeded = await onInvite(trimmedEmail, role);
        if (succeeded) {
            onClose();
        }
    };

    return (
        <Modal isOpen={isOpen} onOpenChange={handleOpenChange}>
            <Modal.Backdrop>
                <Modal.Container placement="center" size="sm">
                    <Modal.Dialog aria-label="Пригласить участника">
                        <Modal.Header>
                            <Modal.Heading>Пригласить участника</Modal.Heading>
                        </Modal.Header>
                        <Modal.Body>
                            <form
                                id="property-invite-form"
                                className={styles.form}
                                onSubmit={handleSubmit}
                            >
                                <p className={styles.hint}>
                                    Если пользователь уже зарегистрирован, доступ откроется сразу.
                                    Иначе мы отправим приглашение на email.
                                </p>
                                <TextField
                                    label="Email"
                                    type="email"
                                    placeholder="user@example.com"
                                    value={email}
                                    onChange={(e) => setEmail(e.currentTarget.value)}
                                    error={emailError}
                                    required
                                    fullWidth
                                />
                                <Select
                                    label="Роль"
                                    value={role}
                                    options={memberRoleOptions}
                                    onChange={(v) => setRole(v)}
                                />
                            </form>
                        </Modal.Body>
                        <Modal.Footer>
                            <div className={styles.footer}>
                                <Button variant="secondary" onClick={onClose} type="button">
                                    Отмена
                                </Button>
                                <Button
                                    variant="primary"
                                    type="submit"
                                    form="property-invite-form"
                                    loading={isSubmitting}
                                    disabled={isSubmitAttempted && !isEmailValid}
                                >
                                    Пригласить
                                </Button>
                            </div>
                        </Modal.Footer>
                    </Modal.Dialog>
                </Modal.Container>
            </Modal.Backdrop>
        </Modal>
    );
}
