'use client';

import {useCallback, useMemo, useState, type FormEvent, type JSX} from 'react';
import {useParams, useRouter} from 'next/navigation';
import {notify} from '@/shared/lib/notifications';
import {ROUTES} from '@/shared/config/routes';
import {Button} from '@/shared/ui/button';
import {TextField} from '@/shared/ui/text-field';
import {Select} from '@/shared/ui/select';
import {IconButton} from '@/shared/ui/icon-button';
import {PageHeader} from '@/shared/ui/page-header';
import {Cancel} from '@/shared/assets/icons';
import {
    useCreatePropertyAccessMember,
    useDeletePropertyAccessMember,
    useLeaveProperty,
    usePropertyAccessMembers,
    useUpdatePropertyAccessMember,
} from '@/features/access/api';
import {useMe} from '@/features/auth/api/hooks';
import {memberRoleLabel, memberRoleOptions, type MemberRole} from '@/features/access/lib/roles';
import type {PropertyAccessMember} from '@/entities/access/model/types';
import styles from './PropertyAccessPage.module.css';

const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export function PropertyAccessPage(): JSX.Element {
    const params = useParams<{ id: string }>();
    const propertyId = params.id ?? '';
    const router = useRouter();

    const membersQuery = usePropertyAccessMembers(propertyId);
    const createMember = useCreatePropertyAccessMember(propertyId);
    const updateMember = useUpdatePropertyAccessMember(propertyId);
    const deleteMember = useDeletePropertyAccessMember(propertyId);
    const leaveProperty = useLeaveProperty(propertyId);
    const meQuery = useMe();

    const [userIdInput, setUserIdInput] = useState('');
    const [newRole, setNewRole] = useState<MemberRole>('full_access');
    const [isSubmitAttempted, setIsSubmitAttempted] = useState(false);

    const members = useMemo(() => membersQuery.data ?? [], [membersQuery.data]);

    const isUserIdValid = UUID_PATTERN.test(userIdInput.trim());
    const canSubmit = isUserIdValid && !createMember.isPending;
    const userIdError = isSubmitAttempted && !isUserIdValid
        ? 'Введите идентификатор пользователя (UUID)'
        : undefined;

    const handleAdd = useCallback(
        async (event: FormEvent<HTMLFormElement>) => {
            event.preventDefault();
            setIsSubmitAttempted(true);
            if (!canSubmit) return;
            try {
                await createMember.mutateAsync({
                    userId: userIdInput.trim(),
                    role: newRole,
                });
                notify.scenarios.access.memberAdded();
                setUserIdInput('');
                setIsSubmitAttempted(false);
            } catch (error: unknown) {
                notify.scenarios.access.addError(error);
            }
        },
        [canSubmit, createMember, newRole, userIdInput],
    );

    const handleChangeRole = useCallback(
        async (member: PropertyAccessMember, role: MemberRole) => {
            if (member.id === null || member.role === role) return;
            try {
                await updateMember.mutateAsync({memberId: member.id, role});
                notify.scenarios.access.roleChanged();
            } catch (error: unknown) {
                notify.scenarios.access.roleChangeError(error);
            }
        },
        [updateMember],
    );

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
        () => members.some((m) => !m.isOwner && m.userId === currentUserId),
        [members, currentUserId],
    );

    const isLoading = membersQuery.isPending;
    const hasError = membersQuery.isError;

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

            <form className={styles.addForm} onSubmit={handleAdd}>
                <h2 className={styles.sectionTitle}>Добавить участника</h2>
                <p className={styles.hint}>
                    Введите идентификатор зарегистрированного пользователя. Приглашение
                    по email появится позже.
                </p>
                <div className={styles.fields}>
                    <TextField
                        label="Идентификатор пользователя"
                        placeholder="00000000-0000-0000-0000-000000000000"
                        value={userIdInput}
                        onChange={(e) => setUserIdInput(e.currentTarget.value)}
                        error={userIdError}
                        fullWidth
                    />
                    <Select
                        label="Роль"
                        value={newRole}
                        options={memberRoleOptions as unknown as {value: string; label: string}[]}
                        onChange={(v) => setNewRole(v as MemberRole)}
                    />
                </div>
                <Button
                    type="submit"
                    variant="primary"
                    size="large"
                    loading={createMember.isPending}
                    disabled={!canSubmit}
                >
                    Добавить
                </Button>
            </form>

            <section className={styles.members}>
                <h2 className={styles.sectionTitle}>Участники</h2>
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
                        {members.map((member) => (
                            <li key={member.userId} className={styles.member}>
                                <div className={styles.memberInfo}>
                                    <span className={styles.memberName}>
                                        {member.displayName || 'Без имени'}
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
                                </div>
                                {!member.isOwner && member.id !== null && (
                                    <div className={styles.memberActions}>
                                        <Select
                                            value={member.role as MemberRole}
                                            options={memberRoleOptions as unknown as {value: string; label: string}[]}
                                            onChange={(v) =>
                                                handleChangeRole(member, v as MemberRole)
                                            }
                                            disabled={updateMember.isPending}
                                        />
                                        <Button
                                            variant="secondary"
                                            size="medium"
                                            loading={deleteMember.isPending}
                                            onClick={() => handleRevoke(member)}
                                        >
                                            Отозвать
                                        </Button>
                                    </div>
                                )}
                            </li>
                        ))}
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
        </div>
    );
}
