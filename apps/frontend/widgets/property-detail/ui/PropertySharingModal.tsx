'use client';

import {useEffect, useRef, useState, type ChangeEvent, type JSX} from 'react';
import {Modal} from '@heroui/react';
import clsx from 'clsx';
import {notify} from '@/shared/lib/notifications';
import {ROUTES} from '@/shared/config/routes';
import {ApiError} from '@/shared/api/errors';
import {Button} from '@/shared/ui/button';
import {TextField} from '@/shared/ui/text-field';
import {Select, type SelectOption} from '@/shared/ui/select';
import {Cancel, SmallArrowDown} from '@/shared/assets/icons';
import {ACCESS_ROLE_LABELS} from '@/entities/access';
import {accessRoleIcon} from '@/entities/access';
import {AccessRoleBadge} from '@/entities/access';
import {
    useCancelPropertyAccessInvitation,
    useDeletePropertyAccessMember,
    useInvitePropertyAccessMember,
    usePropertyAccessMembers,
    useResendPropertyAccessInvitation,
    useUpdatePropertyAccessInvitation,
    useUpdatePropertyAccessMember,
} from '@/features/access';
import {useMe} from '@/features/auth';
import type {MemberRole} from '@/features/access';
import type {
    AccessMemberStatus,
    AccessRole,
    PropertyAccessMember,
} from '@/entities/access';
import styles from './PropertySharingModal.module.css';

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

const RESEND_COOLDOWN_MS = 24 * 60 * 60 * 1000;

function resendCooldownHours(lastSentAt: string | null | undefined): number {
    if (!lastSentAt) return 0;
    const remaining = RESEND_COOLDOWN_MS - (Date.now() - new Date(lastSentAt).getTime());
    return remaining > 0 ? Math.ceil(remaining / (60 * 60 * 1000)) : 0;
}

const MEMBER_STATUS_LABELS: Record<AccessMemberStatus, string> = {
    active: 'Активен',
    pending: 'Ожидает регистрации',
    suspended: 'Превышен лимит объектов',
};

const STATUS_CHIP_CLASS: Record<AccessMemberStatus, string> = {
    active: styles.statusActive ?? '',
    pending: styles.statusPending ?? '',
    suspended: styles.statusSuspended ?? '',
};

const ADD_ROLE_OPTIONS: readonly SelectOption<MemberRole>[] = [
    {value: 'viewer', label: ACCESS_ROLE_LABELS.viewer},
    {value: 'full_access', label: ACCESS_ROLE_LABELS.full_access},
];

type RoleSelectValue = MemberRole | 'revoke';

const ROLE_SELECT_OPTIONS: readonly SelectOption<RoleSelectValue>[] = [
    {value: 'viewer', label: ACCESS_ROLE_LABELS.viewer},
    {value: 'full_access', label: ACCESS_ROLE_LABELS.full_access},
    {value: 'revoke', label: 'Отозвать доступ'},
];

function toMemberRole(role: AccessRole): MemberRole {
    return role === 'full_access' ? 'full_access' : 'viewer';
}

function memberLabel(member: PropertyAccessMember): string {
    if (member.status === 'pending') {
        return member.email ?? '';
    }
    if (member.displayName !== '') {
        return member.displayName;
    }
    if (member.email !== null && member.email !== '') {
        return member.email;
    }
    return 'Без имени';
}

export type PropertySharingModalProps = {
    readonly propertyId: string;
    readonly isOpen: boolean;
    readonly onClose: () => void;
    readonly isArchived?: boolean;
};

export function PropertySharingModal({
    propertyId,
    isOpen,
    onClose,
    isArchived = false,
}: PropertySharingModalProps): JSX.Element {
    const membersQuery = usePropertyAccessMembers(propertyId);
    const inviteMember = useInvitePropertyAccessMember(propertyId);
    const updateMember = useUpdatePropertyAccessMember(propertyId);
    const updateInvitation = useUpdatePropertyAccessInvitation(propertyId);
    const resendInvitation = useResendPropertyAccessInvitation(propertyId);
    const cancelInvitation = useCancelPropertyAccessInvitation(propertyId);
    const deleteMember = useDeletePropertyAccessMember(propertyId);
    const meQuery = useMe();

    const [email, setEmail] = useState('');
    const [addRole, setAddRole] = useState<MemberRole>('viewer');
    const [addError, setAddError] = useState<string | null>(null);
    const [notice, setNotice] = useState<string | null>(null);
    const [copied, setCopied] = useState(false);
    const [wasOpen, setWasOpen] = useState(isOpen);

    if (isOpen !== wasOpen) {
        setWasOpen(isOpen);
        if (!isOpen) {
            setEmail('');
            setAddRole('viewer');
            setAddError(null);
            setNotice(null);
            setCopied(false);
        }
    }

    const members = membersQuery.data ?? [];
    const owner = members.find((member) => member.isOwner);
    const participants = members.filter((member) => !member.isOwner);

    const currentUserId = meQuery.data?.id;
    const currentMember = members.find(
        (member) => member.userId !== null && member.userId === currentUserId,
    );
    const isViewer =
        meQuery.isPending ||
        (currentMember !== undefined && !currentMember.isOwner && currentMember.role === 'viewer');
    const showAddRow = !isArchived && !isViewer;

    const isMemberActionPending =
        updateMember.isPending ||
        updateInvitation.isPending ||
        resendInvitation.isPending ||
        cancelInvitation.isPending ||
        deleteMember.isPending;

    const handleOpenChange = (open: boolean): void => {
        if (!open) {
            onClose();
        }
    };

    const handleEmailChange = (event: ChangeEvent<HTMLInputElement>): void => {
        setEmail(event.currentTarget.value);
        if (addError) {
            setAddError(null);
        }
    };

    const handleAdd = async (): Promise<void> => {
        setNotice(null);
        const trimmedEmail = email.trim();
        if (!trimmedEmail) {
            setAddError('Введите email');
            return;
        }
        if (!EMAIL_PATTERN.test(trimmedEmail)) {
            setAddError('Введите корректный email');
            return;
        }
        try {
            const result = await inviteMember.mutateAsync({email: trimmedEmail, role: addRole});
            setEmail('');
            setAddError(null);
            if (result.status === 'suspended') {
                // У получателя нет свободного слота по тарифу: доступ создан
                // приостановленным — это не ошибка, показываем inline-уведомление.
                setNotice(
                    `У ${trimmedEmail} нет свободного слота по тарифу — доступ создан приостановленным и активируется автоматически, когда слот освободится.`,
                );
            } else if (result.status === 'pending') {
                notify.scenarios.access.invited();
            } else {
                notify.scenarios.access.memberAdded();
            }
        } catch (error: unknown) {
            if (error instanceof ApiError && error.status === 409) {
                setAddError(error.detail || 'Этому адресу уже предоставлен доступ');
                return;
            }
            notify.scenarios.access.inviteError(error);
        }
    };

    const handleRoleChange = async (
        member: PropertyAccessMember,
        next: RoleSelectValue,
    ): Promise<void> => {
        if (member.id === null) return;
        if (next === 'revoke') {
            try {
                if (member.status === 'pending') {
                    await cancelInvitation.mutateAsync(member.id);
                    notify.scenarios.access.invitationCancelled();
                } else {
                    await deleteMember.mutateAsync(member.id);
                    notify.scenarios.access.revoked();
                }
            } catch (error: unknown) {
                if (member.status === 'pending') {
                    notify.scenarios.access.cancelInvitationError(error);
                } else {
                    notify.scenarios.access.revokeError(error);
                }
            }
            return;
        }
        if (member.role === next) return;
        try {
            if (member.status === 'pending') {
                await updateInvitation.mutateAsync({invitationId: member.id, role: next});
            } else {
                await updateMember.mutateAsync({memberId: member.id, role: next});
            }
            notify.scenarios.access.roleChanged();
        } catch (error: unknown) {
            notify.scenarios.access.roleChangeError(error);
        }
    };

    const handleResend = async (member: PropertyAccessMember): Promise<void> => {
        if (member.id === null) return;
        try {
            await resendInvitation.mutateAsync(member.id);
            notify.scenarios.access.invitationResent();
        } catch (error: unknown) {
            notify.scenarios.access.resendError(error);
        }
    };

    const copiedTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

    useEffect(() => {
        return () => {
            if (copiedTimeoutRef.current !== null) {
                clearTimeout(copiedTimeoutRef.current);
            }
        };
    }, []);

    const handleCopyLink = (): void => {
        // Ссылка не даёт прав — только для удобства. В небезопасном контексте
        // (http) clipboard у навигатора отсутствует, хотя DOM-тип считает его
        // всегда доступным — расширение типа сохраняет runtime-проверку.
        const clipboard = navigator.clipboard as Clipboard | undefined;
        clipboard
            ?.writeText(`${window.location.origin}${ROUTES.property(propertyId)}`)
            .catch(() => {
                // clipboard может быть недоступен — подсказку всё равно показываем
            });
        setCopied(true);
        if (copiedTimeoutRef.current !== null) {
            clearTimeout(copiedTimeoutRef.current);
        }
        copiedTimeoutRef.current = setTimeout(() => setCopied(false), 2000);
    };

    const isLoading = membersQuery.isPending;
    const hasError = membersQuery.isError;

    return (
        <Modal isOpen={isOpen} onOpenChange={handleOpenChange}>
            <Modal.Backdrop>
                <Modal.Container placement="center" size="lg">
                    <Modal.Dialog aria-label="Совместный доступ">
                        <Modal.Header>
                            <Modal.Heading>Совместный доступ</Modal.Heading>
                        </Modal.Header>
                        <Modal.Body>
                            <div className={styles.body}>
                                {showAddRow && (
                                    <div className={styles.addRow}>
                                        <div className={styles.emailField}>
                                            <TextField
                                                type="email"
                                                placeholder="Добавьте email"
                                                value={email}
                                                onChange={handleEmailChange}
                                                fullWidth
                                                error={addError ?? undefined}
                                            />
                                        </div>
                                        <div className={styles.addRoleSelect}>
                                            <Select<MemberRole>
                                                options={ADD_ROLE_OPTIONS}
                                                value={addRole}
                                                onChange={(value) => setAddRole(value)}
                                                renderTrigger={({isOpen: triggerOpen, onClick}) => (
                                                    <button
                                                        type="button"
                                                        className={styles.addTrigger}
                                                        aria-expanded={triggerOpen}
                                                        aria-haspopup="listbox"
                                                        aria-label={`Роль: ${ACCESS_ROLE_LABELS[addRole]}`}
                                                        onClick={onClick}
                                                    >
                                                        {ACCESS_ROLE_LABELS[addRole]}
                                                        <span
                                                            className={styles.addTriggerChevron}
                                                            aria-hidden="true"
                                                        >
                                                            <SmallArrowDown/>
                                                        </span>
                                                    </button>
                                                )}
                                            />
                                        </div>
                                        <Button
                                            variant="primary"
                                            size="small"
                                            loading={inviteMember.isPending}
                                            onClick={() => void handleAdd()}
                                            type="button"
                                        >
                                            Добавить
                                        </Button>
                                    </div>
                                )}

                                {notice && (
                                    <div className={styles.notice}>
                                        <span className={styles.noticeText}>{notice}</span>
                                        <button
                                            type="button"
                                            className={styles.noticeClose}
                                            aria-label="Скрыть уведомление"
                                            onClick={() => setNotice(null)}
                                        >
                                            <Cancel/>
                                        </button>
                                    </div>
                                )}

                                <h3 className={styles.sectionHeading}>
                                    Пользователи, имеющие доступ
                                </h3>

                                {isLoading && <p className={styles.state}>Загрузка участников…</p>}
                                {hasError && (
                                    <div className={styles.state}>
                                        <p>Не удалось загрузить участников.</p>
                                        <Button
                                            variant="secondary"
                                            size="small"
                                            onClick={() => void membersQuery.refetch()}
                                        >
                                            Повторить
                                        </Button>
                                    </div>
                                )}
                                {!isLoading && !hasError && (
                                    <ul className={styles.list}>
                                        {owner && (
                                            <li className={clsx(styles.row, styles.ownerRow)}>
                                                <div className={styles.rowMain}>
                                                    <span className={styles.email}>
                                                        {owner.email ?? owner.displayName}
                                                    </span>
                                                </div>
                                                <span className={styles.ownerLabel}>Владелец</span>
                                            </li>
                                        )}
                                        {participants.map((member) => {
                                            const role = toMemberRole(member.role);
                                            const cooldownHours =
                                                member.status === 'pending'
                                                    ? resendCooldownHours(member.lastSentAt)
                                                    : 0;
                                            return (
                                                <li
                                                    key={
                                                        member.id ??
                                                        member.userId ??
                                                        member.email ??
                                                        ''
                                                    }
                                                    className={styles.row}
                                                >
                                                    <div className={styles.rowMain}>
                                                        <span className={styles.email}>
                                                            {memberLabel(member)}
                                                        </span>
                                                        <span className={styles.statusLine}>
                                                            <span
                                                                className={clsx(
                                                                    styles.statusChip,
                                                                    STATUS_CHIP_CLASS[
                                                                        member.status
                                                                    ]
                                                                )}
                                                            >
                                                                {
                                                                    MEMBER_STATUS_LABELS[
                                                                        member.status
                                                                    ]
                                                                }
                                                            </span>
                                                            {!isViewer &&
                                                                member.status === 'pending' &&
                                                                (cooldownHours > 0 ? (
                                                                    <span className={styles.cooldown}>
                                                                        можно переотправить через{' '}
                                                                        {cooldownHours} ч
                                                                    </span>
                                                                ) : (
                                                                    <button
                                                                        type="button"
                                                                        className={
                                                                            styles.resendButton
                                                                        }
                                                                        disabled={
                                                                            resendInvitation.isPending
                                                                        }
                                                                        onClick={() =>
                                                                            void handleResend(
                                                                                member,
                                                                            )
                                                                        }
                                                                    >
                                                                        Переотправить
                                                                    </button>
                                                                ))}
                                                        </span>
                                                    </div>
                                                    <div className={styles.roleSelect}>
                                                        {isViewer ? (
                                                            <AccessRoleBadge role={role}/>
                                                        ) : (
                                                            <Select<RoleSelectValue>
                                                                options={ROLE_SELECT_OPTIONS}
                                                                value={role}
                                                                onChange={(value) =>
                                                                    void handleRoleChange(
                                                                        member,
                                                                        value,
                                                                    )
                                                                }
                                                                dropdownAlign="right"
                                                                dropdownClassName={styles.roleDropdown}
                                                                renderTrigger={({
                                                                    isOpen: triggerOpen,
                                                                    onClick,
                                                                }) => (
                                                                    <button
                                                                        type="button"
                                                                        className={
                                                                            styles.roleTrigger
                                                                        }
                                                                        title={
                                                                            ACCESS_ROLE_LABELS[role]
                                                                        }
                                                                        aria-expanded={triggerOpen}
                                                                        aria-haspopup="listbox"
                                                                        aria-label={`Роль: ${ACCESS_ROLE_LABELS[role]}`}
                                                                        disabled={isMemberActionPending}
                                                                        onClick={onClick}
                                                                    >
                                                                        {accessRoleIcon(role)}
                                                                    </button>
                                                                )}
                                                            />
                                                        )}
                                                    </div>
                                                </li>
                                            );
                                        })}
                                    </ul>
                                )}
                            </div>
                        </Modal.Body>
                        <Modal.Footer>
                            <div className={styles.footer}>
                                <div className={styles.copyWrap}>
                                    <Button
                                        variant="clear"
                                        size="small"
                                        onClick={handleCopyLink}
                                        type="button"
                                    >
                                        Копировать ссылку
                                    </Button>
                                    {copied && (
                                        <span className={styles.copiedHint}>Скопировано</span>
                                    )}
                                </div>
                                <Button
                                    variant="primary"
                                    size="small"
                                    onClick={onClose}
                                    type="button"
                                >
                                    Готово
                                </Button>
                            </div>
                        </Modal.Footer>
                    </Modal.Dialog>
                </Modal.Container>
            </Modal.Backdrop>
        </Modal>
    );
}
