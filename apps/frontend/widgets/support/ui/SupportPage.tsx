import type {JSX} from 'react';
import NextLink from 'next/link';
import {Card} from '@heroui/react/card';
import {PageHeader} from '@/shared/ui/page-header';
import {PageShell} from '@/shared/ui/page-shell';
import {Icon} from '@/shared/ui/icon';
import {ArrowRight} from '@/shared/assets/icons';
import styles from './SupportPage.module.css';

type Contact = {
    readonly label: string;
    readonly value: string;
    readonly href: string;
};

const contacts: Contact[] = [
    {
        label: 'Почта поддержки',
        value: 'hello@rentlee.ru',
        href: 'mailto:hello@rentlee.ru',
    },
    {
        label: 'Телефон',
        value: '8 (800) 555-35-35',
        href: 'tel:88005553535',
    },
];

export function SupportPage(): JSX.Element {
    return (
        <PageShell>
            <PageHeader title="Поддержка"/>
            <div className={styles.root}>
                <Card className={styles.helpCard}>
                    <h2 className={styles.helpTitle}>Нужна помощь?</h2>
                    <p className={styles.helpText}>
                        Напишите нам на почту или позвоните. Мы на связи ежедневно с 9:00 до
                        21:00 по московскому времени.
                    </p>
                </Card>

                <nav className={styles.contacts} aria-label="Контакты поддержки">
                    {contacts.map((contact) => (
                        <NextLink
                            key={contact.href}
                            href={contact.href}
                            className={styles.contactLink}
                            prefetch={false}
                        >
                            <Card className={styles.contactCard}>
                                <div className={styles.contactLeft}>
                                    <span className={styles.contactLabel}>{contact.label}</span>
                                </div>
                                <div className={styles.contactRight}>
                                    <span className={styles.contactValue}>{contact.value}</span>
                                    <Icon size="s">
                                        <ArrowRight/>
                                    </Icon>
                                </div>
                            </Card>
                        </NextLink>
                    ))}
                </nav>
            </div>
        </PageShell>
    );
}
