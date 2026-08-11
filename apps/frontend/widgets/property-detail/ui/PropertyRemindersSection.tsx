'use client';

import type {JSX} from 'react';
import Link from 'next/link';
import {ROUTES} from '@/shared/config/routes';
import {LinkButton} from '@/shared/ui/link-button';
import {SectionHeader} from '@/widgets/dashboard/ui/SectionHeader';
import {PropertyDetailSection} from './PropertyDetailSection';
import {EmptyState} from '@/shared/ui/empty-state';
import {Icon} from '@/shared/ui/icon';
import {ArrowRight, Bell, Plus} from '@/shared/assets/icons';
import {formatReminderDateTime} from '@/shared/lib/datetime';
import {PERIODICITY_LABELS} from '@/features/free-reminders/model/types';
import {useUpcomingFreeReminders} from '@/features/free-reminders/api';
import styles from './PropertyRemindersSection.module.css';

export type PropertyRemindersSectionProps = {
    readonly propertyId: string;
    readonly isArchived?: boolean;
};

export function PropertyRemindersSection({
                                            propertyId,
                                            isArchived = false,
                                        }: PropertyRemindersSectionProps): JSX.Element {
    const {data, isPending, isError} = useUpcomingFreeReminders(propertyId);
    const items = data?.items;

    return (
        <PropertyDetailSection>
            <div className={styles.headerRow}>
                <SectionHeader title="Напоминания" count={items?.length}/>
                <Link href={ROUTES.calendar} className={styles.calendarLink}>
                    В календарь
                    <Icon size="xs"><ArrowRight/></Icon>
                </Link>
            </div>

            {isPending && <p className={styles.notice}>Загрузка…</p>}

            {isError && <p className={styles.notice}>Не удалось загрузить напоминания</p>}

            {!isPending && !isError && (!items || items.length === 0) && (
                <EmptyState
                    icon={<Bell/>}
                    subtitle="По этому объекту пока нет напоминаний"
                    actionHref={`${ROUTES.freeReminderNew}?propertyId=${propertyId}`}
                    actionText="Добавить напоминание"
                />
            )}

            {!isPending && !isError && items && items.length > 0 && (
                <>
                    <ul className={styles.list}>
                        {items.map((reminder) => {
                            const {date, time} = formatReminderDateTime(reminder.trigger_at);
                            return (
                                <li key={`${reminder.free_reminder_id}__${reminder.trigger_at}`}>
                                    <Link
                                        href={ROUTES.freeReminder(reminder.free_reminder_id)}
                                        className={styles.row}
                                    >
                                        <span className={styles.rowMain}>
                                            <span className={styles.rowName}>{reminder.title}</span>
                                            <span className={styles.rowMeta}>{date} в {time}</span>
                                        </span>
                                        <span className={styles.badge}>
                                            {PERIODICITY_LABELS[reminder.periodicity]}
                                        </span>
                                    </Link>
                                </li>
                            );
                        })}
                    </ul>
                    <LinkButton
                        href={`${ROUTES.freeReminderNew}?propertyId=${propertyId}`}
                        variant="secondary"
                        fullWidth
                        disabled={isArchived}
                        title={isArchived ? 'Объект в архиве' : undefined}
                        leftIcon={<Plus/>}
                    >
                        Добавить напоминание
                    </LinkButton>
                </>
            )}
        </PropertyDetailSection>
    );
}
