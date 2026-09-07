'use client';

import type {JSX} from 'react';
import NextLink from 'next/link';
import {Card} from '@heroui/react/card';
import {Icon} from '@/shared/ui/icon';
import {ArrowRight} from '@/shared/assets/icons';
import {ROUTES} from '@/shared/config/routes';
import styles from './ProfileMenu.module.css';

type MenuItem = {
    readonly title: string;
    readonly href: string;
};

const menuItems: MenuItem[] = [
    {title: 'Задачи', href: ROUTES.tasks},
    {title: 'Мои данные', href: ROUTES.profilePersonal},
    {title: 'Уведомления', href: ROUTES.profileNotifications},
    {title: 'Аккаунт', href: '/profile/account'},
    {title: 'Тариф', href: '/profile/tariff'},
    {title: 'Поддержка', href: '/support'},
    {title: 'Информация', href: '/profile/info'},
];

export function ProfileMenu(): JSX.Element {
    return (
        <nav className={styles.root} aria-label="Разделы профиля">
            {menuItems.map((item) => (
                <NextLink
                    key={item.href}
                    href={item.href}
                    className={styles.link}
                    aria-label={`Перейти в раздел «${item.title}»`}
                >
                    <Card className={styles.card}>
                        <span className={styles.title}>{item.title}</span>
                        <Icon size="s">
                            <ArrowRight/>
                        </Icon>
                    </Card>
                </NextLink>
            ))}
        </nav>
    );
}
