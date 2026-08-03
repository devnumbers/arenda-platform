// PROTOTYPE — throwaway, issue #106
'use client';

import { type JSX, useState } from 'react';
import NextLink from 'next/link';
import clsx from 'clsx';
import { ArrowLeft, ArrowRight, Bell, BoldWallet, Key, Plus } from '@/shared/assets/icons';
import { Icon } from '@/shared/ui/icon';
import { IconButton } from '@/shared/ui/icon-button';
import { LinkButton } from '@/shared/ui/link-button';
import { PageHeader } from '@/shared/ui/page-header';
import { PROTOTYPE_REMINDER_CREATE_HREF, PROTOTYPE_REMINDER_ITEM_HREF } from '../../model/variant';
import {
  comparePrototypeReminders,
  formatPrototypeDateShort,
  groupRemindersByProperty,
  pluralizePrototypeReminders,
  prototypeCalendarKindLabels,
  prototypeCalendarStatusLabels,
  prototypeMonthTitle,
  prototypeReminderIsRecurring,
  prototypeReminderStatus,
  PROTOTYPE_TODAY,
  remindersForMonth,
  type PrototypeCalendarKind,
  type PrototypeCalendarPropertyGroup,
  type PrototypeCalendarReminder,
} from '../../model/mocks';
import styles from './VariantCObjectFeed.module.css';

export const variantName = 'Лента по объектам';

const STUB_TITLES: Record<'operation' | 'system', string> = {
  operation: 'Страница операции — вне прототипа',
  system: 'Страница аренды — вне прототипа',
};

const KIND_ICONS: Record<PrototypeCalendarKind, JSX.Element> = {
  free: <Bell />,
  operation: <BoldWallet />,
  system: <Key />,
};

function todayCursor(): { readonly year: number; readonly monthIndex: number } {
  const today = new Date(`${PROTOTYPE_TODAY}T00:00:00`);
  return { year: today.getFullYear(), monthIndex: today.getMonth() };
}

// Ближайшее предстоящее напоминание секции (или последнее, если всё в прошлом).
function groupSortKey(group: PrototypeCalendarPropertyGroup): string {
  const upcoming = group.reminders.find((reminder) => reminder.date >= PROTOTYPE_TODAY);
  return (upcoming ?? group.reminders[group.reminders.length - 1]).date;
}

// Просроченные — первыми внутри секции, дальше по хронологии.
function compareSectionRows(a: PrototypeCalendarReminder, b: PrototypeCalendarReminder): number {
  const aOverdue = a.operationStatus === 'overdue' ? 0 : 1;
  const bOverdue = b.operationStatus === 'overdue' ? 0 : 1;
  if (aOverdue !== bOverdue) {
    return aOverdue - bOverdue;
  }
  return comparePrototypeReminders(a, b);
}

function statusChipClass(reminder: PrototypeCalendarReminder): string {
  return clsx(
    styles.statusChip,
    reminder.operationStatus === 'overdue' && styles.statusChipOverdue,
    reminder.operationStatus === 'due_soon' && styles.statusChipDueSoon,
    reminder.systemType !== undefined && styles.statusChipSystem,
  );
}

export function VariantCObjectFeed(): JSX.Element {
  const [cursor, setCursor] = useState(todayCursor);

  const yearMonth = `${cursor.year}-${String(cursor.monthIndex + 1).padStart(2, '0')}`;
  const monthReminders = remindersForMonth(yearMonth);

  const freeCount = monthReminders.filter((reminder) => reminder.kind === 'free').length;
  const operationCount = monthReminders.filter((reminder) => reminder.kind === 'operation').length;
  const systemCount = monthReminders.filter((reminder) => reminder.kind === 'system').length;

  const groups = [...groupRemindersByProperty(monthReminders)].sort((a, b) => {
    if (a.propertyId === null) {
      return 1;
    }
    if (b.propertyId === null) {
      return -1;
    }
    return groupSortKey(a).localeCompare(groupSortKey(b));
  });

  const shiftMonth = (delta: number) => {
    setCursor((prev) => {
      const total = prev.year * 12 + prev.monthIndex + delta;
      return { year: Math.floor(total / 12), monthIndex: ((total % 12) + 12) % 12 };
    });
  };

  const handleStubClick = (reminder: PrototypeCalendarReminder) => {
    console.log('[prototype#106] variant C — переход вне прототипа', reminder.kind, reminder.id);
  };

  return (
    <div className={styles.root}>
      <PageHeader
        title="Календарь"
        actions={
          <LinkButton
            href={PROTOTYPE_REMINDER_CREATE_HREF}
            variant="primary"
            size="medium"
            leftIcon={<Plus />}
          >
            Создать напоминание
          </LinkButton>
        }
      />

      <div className={styles.toolbar}>
        <div className={styles.monthNav}>
          <IconButton
            variant="secondary"
            size="small"
            icon={<ArrowLeft />}
            aria-label="Предыдущий месяц"
            onClick={() => shiftMonth(-1)}
          />
          <h2 className={styles.monthTitle}>{prototypeMonthTitle(cursor.year, cursor.monthIndex)}</h2>
          <IconButton
            variant="secondary"
            size="small"
            icon={<ArrowRight />}
            aria-label="Следующий месяц"
            onClick={() => shiftMonth(1)}
          />
        </div>
        <p className={styles.summary}>
          {monthReminders.length} {pluralizePrototypeReminders(monthReminders.length)}: {freeCount}{' '}
          свободных, {operationCount} по операциям, {systemCount} системных
        </p>
      </div>

      {groups.length === 0 ? (
        <p className={styles.empty}>Нет напоминаний</p>
      ) : (
        <div className={styles.feed}>
          {groups.map((group) => {
            const rows = [...group.reminders].sort(compareSectionRows);
            return (
              <section key={group.propertyId ?? 'orphans'} className={styles.section}>
                <div className={styles.sectionHeader}>
                  <h3 className={styles.sectionTitle}>{group.propertyName}</h3>
                  <p className={styles.sectionCount}>
                    {group.reminders.length} {pluralizePrototypeReminders(group.reminders.length)}
                  </p>
                </div>
                <ul className={styles.sectionList}>
                  {rows.map((reminder) => {
                    const status = prototypeReminderStatus(reminder);
                    const isOverdue = reminder.operationStatus === 'overdue';
                    const rowBody = (
                      <>
                        <span className={styles.rowWhen}>
                          <span className={styles.rowDate}>
                            {formatPrototypeDateShort(reminder.date)}
                            {reminder.date === PROTOTYPE_TODAY && (
                              <span className={styles.todayBadge}>Сегодня</span>
                            )}
                          </span>
                          <span className={styles.rowTime}>{reminder.time ?? ''}</span>
                        </span>
                        <span className={styles.rowKind}>
                          <Icon size="xs">{KIND_ICONS[reminder.kind]}</Icon>
                          {prototypeCalendarKindLabels[reminder.kind]}
                        </span>
                        <span className={styles.rowTitle}>
                          {prototypeReminderIsRecurring(reminder) && (
                            <span className={styles.rowRepeat} aria-hidden="true">
                              ↻
                            </span>
                          )}
                          {reminder.title}
                        </span>
                        {status && (
                          <span className={statusChipClass(reminder)}>
                            {prototypeCalendarStatusLabels[status]}
                          </span>
                        )}
                      </>
                    );
                    const rowClass = clsx(styles.row, isOverdue && styles.rowOverdue);
                    return (
                      <li key={reminder.id}>
                        {reminder.kind === 'free' ? (
                          <NextLink href={PROTOTYPE_REMINDER_ITEM_HREF} className={rowClass}>
                            {rowBody}
                          </NextLink>
                        ) : (
                          <NextLink
                            href="#"
                            title={STUB_TITLES[reminder.kind]}
                            className={rowClass}
                            onClick={(event) => {
                              event.preventDefault();
                              handleStubClick(reminder);
                            }}
                          >
                            {rowBody}
                          </NextLink>
                        )}
                      </li>
                    );
                  })}
                </ul>
              </section>
            );
          })}
        </div>
      )}
    </div>
  );
}
