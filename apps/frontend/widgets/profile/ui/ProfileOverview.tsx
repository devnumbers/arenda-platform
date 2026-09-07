'use client';

import type {JSX} from 'react';
import {useRouter} from 'next/navigation';
import {Card} from '@heroui/react/card';
import {Skeleton} from '@heroui/react/skeleton';
import {notify} from '@/shared/lib/notifications';
import {Button} from '@/shared/ui/button';
import {Icon} from '@/shared/ui/icon';
import {BoldStar} from '@/shared/assets/icons';
import {useLogout, useMe} from '@/features/auth';
import {ROUTES} from '@/shared/config/routes';
import type {User} from '@/entities/user';
import {getTariffLabel} from '@/entities/user';
import {ProfileMenu} from './ProfileMenu';
import styles from './ProfileOverview.module.css';

function getFullName(user: User): string {
    const parts = [user.surname, user.name, user.patronymic].filter(Boolean);
    return parts.join(' ') || "Пользователь";
}

function UserCardSkeleton(): JSX.Element {
    return (
        <Card className={styles.userCard}>
            <Skeleton className={styles.nameSkeleton}/>
            <Skeleton className={styles.fieldSkeleton}/>
            <Skeleton className={styles.fieldSkeleton}/>
        </Card>
    );
}

export function ProfileOverview(): JSX.Element {
    const router = useRouter();
    const {data: me, isError, refetch} = useMe();
    const logout = useLogout();

    const handleLogout = () => {
        logout.mutate(undefined, {
            onSuccess: () => {
                router.push(ROUTES.login);
            },
            onError: (error) => {
                notify.scenarios.profile.logoutError(error);
            },
        });
    };

    return (
        <div className={styles.page}>
            {isError ? (
                <div className={styles.error}>
                    <p className={styles.errorText}>Не удалось загрузить профиль</p>
                    <Button onClick={() => void refetch()} variant="secondary">
                        Повторить
                    </Button>
                </div>
            ) : !me ? (
                <UserCardSkeleton/>
            ) : (
                <Card className={styles.userCard}>
                    <div className={styles.userHeader}>
                        <span className={styles.userName}>{getFullName(me)}</span>
                        <span className={styles.tariffBadge}>
                              <Icon size="s">
                                <BoldStar className="text-error"/>
                              </Icon>
                                <span>{getTariffLabel(me.subscription?.tariff.name)}</span>
                        </span>
                    </div>
                    <p className={styles.userField}>{me.phone}</p>
                </Card>
            )}

            <ProfileMenu/>

            <Button
                variant="secondary"
                fullWidth
                onClick={handleLogout}
                loading={logout.isPending}
            >
                Выйти
            </Button>
        </div>
    );
}
