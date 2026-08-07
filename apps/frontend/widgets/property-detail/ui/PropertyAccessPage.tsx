'use client';

import {useCallback, useMemo, useState, type JSX} from 'react';
import {useParams, useRouter} from 'next/navigation';
import {notify} from '@/shared/lib/notifications';
import {ROUTES} from '@/shared/config/routes';
import {Button} from '@/shared/ui/button';
import {Select} from '@/shared/ui/select';
import {IconButton} from '@/shared/ui/icon-button';
import {PageHeader} from '@/shared/ui/page-header';
import {ConfirmModal} from '@/shared/ui/confirm-modal';
import {Cancel} from '@/shared/assets/icons';
import {
    useCancelPropertyAccessInvitation,
    useDeletePropertyAccessMember,
    useInvitePropertyAccessMember,
    useLeaveProperty,
    usePropertyAccessMembers,
    useResendPropertyAccessInvitation,
    useUpdatePropertyAccessInvitation,
    useUpdatePropertyAccessMember,
} from '@/features/access/api';
import {useMe} from '@/features/auth/api/hooks';
import {useProperty} from '@/features/properties/api/hooks';
import {memberRoleLabel, memberRoleOptions, type MemberRole} from '@/features/access/lib/roles';
import type {PropertyAccessMember} from '@/entities/access/model/types';
import {PropertyInviteModal} from './PropertyInviteModal';
import styles from './PropertyAccessPage.module.css';

const RESEND_COOLDOWN_MS = 24 * 60 * 60 * 1000;

function resendCooldownHours(lastSentAt: string | null | undefined): number {
    if (!lastSentAt) return 0;
    const remaining = RESEND_COOLDOWN_MS - (Date.now() - new Date(lastSentAt).getTime());
    return remaining > 0 ? Math.ceil(remaining / (60 * 60 * 1000)) : 0;
}

export function PropertyAccessPage(): JSX.Element {
    const params = useParams<{ id: string }>();
    const propertyId = params.id ?? '';
    const router = useRouter();

    const membersQuery = usePropertyAccessMembers(propertyId);
    const propertyQuery = useProperty(propertyId);
    const inviteMember = useInvitePropertyAccessMember(propertyId);
    const updateMember = useUpdatePropertyAccessMember(propertyId);
    const updateInvitation = useUpdatePropertyAccessInvitation(propertyId);
    const resendInvitation = useResendPropertyAccessInvitation(propertyId);
    const cancelInvitation = useCancelPropertyAccessInvitation(propertyId);
    const deleteMember = useDeletePropertyAccessMember(propertyId);
    const leaveProperty = useLeaveProperty(propertyId);
    const meQuery = useMe();

    const [isInviteOpen, setIsInviteOpen] = useState(false);
    const [cancelTarget, setCancelTarget] = useState<PropertyAccessMember | null>(null);

    const members = useMemo(() => membersQuery.data ?? [], [membersQuery.data]);

    const handleInvite = useCallback(
        async (email: string, role: MemberRole): Promise<boolean> => {
            try {
                const result = await inviteMember.mutateAsync({email, role});
                if (result.status === 'pending') {
                    notify.scenarios.access.invited();
                } else {
                    notify.scenarios.access.memberAdded();
                }
                return true;
            } catch (error: unknown) {
                notify.scenarios.access.inviteError(error);
                return false;
            }
        },
        [inviteMember],
    );

    const handleChangeRole = useCallback(
        async (member: PropertyAccessMember, role: MemberRole) => {
            if (member.id === null || member.role === role) return;
            try {
                if (member.status === 'pending') {
                    await updateInvitation.mutateAsync({invitationId: member.id, role});
                } else {
                    await updateMember.mutateAsync({memberId: member.id, role});
                }
                notify.scenarios.access.roleChanged();
            } catch (error: unknown) {
                notify.scenarios.access.roleChangeError(error);
            }
        },
        [updateInvitation, updateMember],
    );

    const handleResend = useCallback(
        async (member: PropertyAccessMember) => {
            if (member.id === null) return;
            try {
                await resendInvitation.mutateAsync(member.id);
                notify.scenarios.access.invitationResent();
            } catch (error: unknown) {
                notify.scenarios.access.resendError(error);
            }
        },
        [resendInvitation],
    );

    const handleCancelInvitation = useCallback(async () => {
        if (cancelTarget?.id == null) return;
        try {
            await cancelInvitation.mutateAsync(cancelTarget.id);
            notify.scenarios.access.invitationCancelled();
        } catch (error: unknown) {
            notify.scenarios.access.cancelInvitationError(error);
        }
    }, [cancelInvitation, cancelTarget]);

    const handleRevoke = useCallback(
        async (member: PropertyAccessMember) => {
            if (member.id === null) return;
            try {
                await deleteMember.mutateAsync(member.id);
                notify.scenarios.access.revoked();
            } catch (error: unknown) {
                notify.scenarios.access.revokeError(error);
            }
        },
        [deleteMember],
    );

    const handleLeave = useCallback(async () => {
        try {
            await leaveProperty.mutateAsync();
            notify.scenarios.access.left();
            // The actor lost access to the property, so navigating back to the
            // object page would now 404 — redirect to the properties list.
            router.replace(ROUTES.properties);
        } catch (error: unknown) {
            notify.scenarios.access.leaveError(error);
        }
    }, [leaveProperty, router]);

    const currentUserId = meQuery.data?.id;
    const canLeave = useMemo(
        () => members.some((m) => !m.isOwner && m.userId !== null && m.userId === currentUserId),
        [members, currentUserId],
    );

    const isLoading = membersQuery.isPending;
    const hasError = membersQuery.isError;
    const isRoleUpdating = updateMember.isPending || updateInvitation.isPending;
    const isArchived = propertyQuery.data?.status === 'archived';

    return (
        <div className={styles.root}>
            <PageHeader
                title="Совместный доступ"
                backHref={ROUTES.property(propertyId)}
                actions={
                    <IconButton
                        variant="secondary"
                        size="large"
                        icon={<Cancel/>}
                        aria-label="Закрыть"
                        onClick={() => router.push(ROUTES.property(propertyId))}
                    />
                }
            />

            <section className={styles.members}>
                <div className={styles.membersHeader}>
                    <h2 className={styles.sectionTitle}>Участники</h2>
                    <Button
                        variant="primary"
                        size="medium"
                        disabled={isArchived}
                        title={
                            isArchived
                                ? 'Нельзя приглашать участников в архивный объект'
                                : undefined
                        }
                        onClick={() => setIsInviteOpen(true)}
                    >
                        Пригласить
                    </Button>
                </div>
                {isLoading && <p className={styles.state}>Загрузка участников…</p>}
                {hasError && (
                    <div className={styles.state}>
                        <p>Не удалось загрузить участников.</p>
                        <Button
                            variant="secondary"
                            size="medium"
                            onClick={() => membersQuery.refetch()}
                        >
                            Повторить
                        </Button>
                    </div>
                )}
                {!isLoading && !hasError && members.length === 0 && (
                    <p className={styles.state}>Участников нет.</p>
                )}
                {!isLoading && !hasError && members.length > 0 && (
                    <ul className={styles.list}>
                        {members.map((member) => {
                            const isPending = member.status === 'pending';
                            const cooldownHours = isPending
                                ? resendCooldownHours(member.lastSentAt)
                                : 0;
                            return (
                                <li
                                    key={member.id ?? member.userId ?? member.email ?? ''}
                                    className={styles.member}
                                >
                                    <div className={styles.memberInfo}>
                                        <span className={styles.memberName}>
                                            {isPending
                                                ? member.email
                                                : member.displayName || 'Без имени'}
                                        </span>
                                        <span className={styles.memberRole}>
                                            {member.isOwner
                                                ? 'Владелец'
                                                : memberRoleLabel(member.role as MemberRole)}
                                        </span>
                                        {member.status === 'suspended' && (
                                            <span className={styles.memberStatus}>
                                                приостановлен: лимит получателя
                                            </span>
                                        )}
                                        {isPending && (
                                            <span className={styles.memberStatusPending}>
                                                ожидает регистрации
                                            </span>
                                        )}
                                    </div>
                                    {!member.isOwner && member.id !== null && (
                                        <div className={styles.memberActions}>
                                            <Select
                                                value={member.role as MemberRole}
                                                options={memberRoleOptions}
                                                onChange={(v) =>
                                                    handleChangeRole(member, v)
                                                }
                                                disabled={isRoleUpdating}
                                            />
                                            {isPending ? (
                                                <>
                                                    <Button
                                                        variant="secondary"
                                                        size="medium"
                                                        loading={resendInvitation.isPending}
                                                        disabled={cooldownHours > 0}
                                                        title={
                                                            cooldownHours > 0
                                                                ? `Повторная отправка доступна через ${cooldownHours} ч.`
                                                                : undefined
                                                        }
                                                        onClick={() => handleResend(member)}
                                                    >
                                                        Переотправить
                                                    </Button>
                                                    <Button
                                                        variant="secondary"
                                                        size="medium"
                                                        onClick={() => setCancelTarget(member)}
                                                    >
                                                        Отменить
                                                    </Button>
                                                </>
                                            ) : (
                                                <Button
                                                    variant="secondary"
                                                    size="medium"
                                                    loading={deleteMember.isPending}
                                                    onClick={() => handleRevoke(member)}
                                                >
                                                    Отозвать
                                                </Button>
                                            )}
                                        </div>
                                    )}
                                </li>
                            );
                        })}
                    </ul>
                )}
            </section>

            {canLeave && (
                <section className={styles.leave}>
                    <Button
                        variant="secondary"
                        size="large"
                        loading={leaveProperty.isPending}
                        onClick={handleLeave}
                    >
                        Покинуть объект
                    </Button>
                </section>
            )}

            <PropertyInviteModal
                isOpen={isInviteOpen}
                isSubmitting={inviteMember.isPending}
                onClose={() => setIsInviteOpen(false)}
                onInvite={handleInvite}
            />

            <ConfirmModal
                isOpen={cancelTarget !== null}
                title="Отменить приглашение?"
                description={
                    cancelTarget?.email
                        ? `Приглашение для ${cancelTarget.email} будет отменено. Пользователь не узнает об этом.`
                        : undefined
                }
                confirmLabel="Отменить приглашение"
                cancelLabel="Назад"
                onClose={() => setCancelTarget(null)}
                onConfirm={() => void handleCancelInvitation()}
            />
        </div>
    );
}
