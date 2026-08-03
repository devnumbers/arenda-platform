// PROTOTYPE — throwaway, issue #106
'use client';

import { type JSX, useState } from 'react';
import NextLink from 'next/link';
import clsx from 'clsx';
import { ArrowLeft, ArrowRight, Bell, Plus } from '@/shared/assets/icons';
import { Button } from '@/shared/ui/button';
import { EmptyState } from '@/shared/ui/empty-state';
import { IconButton } from '@/shared/ui/icon-button';
import { LinkButton } from '@/shared/ui/link-button';
import { PageHeader } from '@/shared/ui/page-header';
import { PROTOTYPE_REMINDER_CREATE_HREF, PROTOTYPE_REMINDER_ITEM_HREF } from '../../model/variant';
import {
  addPrototypeDays,
  formatPrototypeDateWithWeekday,
  prototypeCalendarKindLabels,
  prototypeCalendarStatusLabels,
  prototypePropertyDisplayName,
  prototypeReminderIsRecurring,
  prototypeReminderStatus,
  prototypeWeekDates,
  prototypeWeekdayShort,
  PROTOTYPE_TODAY,
  remindersForDate,
  type PrototypeCalendarReminder,
} from '../../model/mocks';
import styles from './VariantBWeekAgenda.module.css';

export const variantName = 'Неделя + повестка';

const STUB_TITLES: Record<'operation' | 'system', string> = {
  operation: 'Страница операции — вне прототипа',
  system: 'Страница аренды — вне прототипа',
};

function dotClass(reminder: PrototypeCalendarReminder): string {
  return clsx(
    styles.dot,
    reminder.kind === 'free' && styles.dotFree,
    reminder.kind === 'operation' && reminder.operationStatus === 'due_soon' && styles.dotDueSoon,
    reminder.kind === 'operation' && reminder.operationStatus === 'overdue' && styles.dotOverdue,
    reminder.kind === 'system' && styles.dotSystem,
  );
}

function statusChipClass(reminder: PrototypeCalendarReminder): string {
  return clsx(
    styles.statusChip,
    reminder.operationStatus === 'overdue' && styles.statusChipOverdue,
    reminder.operationStatus === 'due_soon' && styles.statusChipDueSoon,
    reminder.systemType !== undefined && styles.statusChipSystem,
  );
}

export function VariantBWeekAgenda(): JSX.Element {
  const [weekAnchor, setWeekAnchor] = useState<string>(PROTOTYPE_TODAY);
  const [selectedDate, setSelectedDate] = useState<string>(PROTOTYPE_TODAY);

  const weekDates = prototypeWeekDates(weekAnchor);
  const selectedReminders = remindersForDate(selectedDate);

  const shiftWeek = (delta: number) => {
    setWeekAnchor((prev) => addPrototypeDays(prev, delta * 7));
  };

  const goToday = () => {
    setWeekAnchor(PROTOTYPE_TODAY);
    setSelectedDate(PROTOTYPE_TODAY);
  };

  const handleStubClick = (reminder: PrototypeCalendarReminder) => {
    console.log('[prototype#106] variant B — переход вне прототипа', reminder.kind, reminder.id);
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

      <div className={styles.weekNav}>
        <IconButton
          variant="secondary"
          size="small"
          icon={<ArrowLeft />}
          aria-label="Предыдущая неделя"
          onClick={() => shiftWeek(-1)}
        />
        <IconButton
          variant="secondary"
          size="small"
          icon={<ArrowRight />}
          aria-label="Следующая неделя"
          onClick={() => shiftWeek(1)}
        />
        <Button variant="secondary" size="small" onClick={goToday}>
          Сегодня
        </Button>
      </div>

      <div className={styles.weekStrip}>
        {weekDates.map((date) => {
          const dayReminders = remindersForDate(date);
          return (
            <button
              key={date}
              type="button"
              className={clsx(
                styles.dayCard,
                date === selectedDate && styles.dayCardSelected,
                date === PROTOTYPE_TODAY && styles.dayCardToday,
              )}
              aria-pressed={date === selectedDate}
              onClick={() => setSelectedDate(date)}
            >
              <span className={styles.dayCardWeekday}>{prototypeWeekdayShort(date)}</span>
              <span className={styles.dayCardNumber}>{Number(date.slice(8, 10))}</span>
              <span className={styles.dayCardDots}>
                {dayReminders.slice(0, 3).map((reminder) => (
                  <span key={reminder.id} className={dotClass(reminder)} aria-hidden="true" />
                ))}
                {dayReminders.length > 0 && (
                  <span className={styles.dayCardCount}>{dayReminders.length}</span>
                )}
              </span>
            </button>
          );
        })}
      </div>

      <section className={styles.agenda} aria-label="Повестка выбранного дня">
        <h2 className={styles.agendaTitle}>{formatPrototypeDateWithWeekday(selectedDate)}</h2>
        {selectedReminders.length === 0 ? (
          <EmptyState
            icon={<Bell />}
            title="Нет напоминаний"
            subtitle="На этот день ничего не запланировано"
            actionNode={
              <LinkButton
                href={PROTOTYPE_REMINDER_CREATE_HREF}
                variant="secondary"
                size="medium"
                leftIcon={<Plus />}
              >
                Создать напоминание
              </LinkButton>
            }
          />
        ) : (
          <ul className={styles.agendaList}>
            {selectedReminders.map((reminder) => {
              const status = prototypeReminderStatus(reminder);
              const rowBody = (
                <>
                  <span className={styles.rowTime}>{reminder.time ?? 'весь день'}</span>
                  <span
                    className={clsx(
                      styles.kindBadge,
                      reminder.kind === 'free' && styles.kindBadgeFree,
                      reminder.kind === 'operation' && styles.kindBadgeOperation,
                      reminder.kind === 'system' && styles.kindBadgeSystem,
                    )}
                  >
                    {prototypeCalendarKindLabels[reminder.kind]}
                  </span>
                  <span className={styles.rowMain}>
                    <span className={styles.rowTitle}>
                      {prototypeReminderIsRecurring(reminder) && (
                        <span className={styles.rowRepeat} aria-hidden="true">
                          ↻
                        </span>
                      )}
                      {reminder.title}
                    </span>
                    <span
                      className={clsx(
                        styles.rowProperty,
                        reminder.propertyId === null && styles.rowPropertyOrphan,
                      )}
                    >
                      {prototypePropertyDisplayName(reminder)}
                    </span>
                  </span>
                  {status && (
                    <span className={statusChipClass(reminder)}>
                      {prototypeCalendarStatusLabels[status]}
                    </span>
                  )}
                </>
              );
              return (
                <li key={reminder.id}>
                  {reminder.kind === 'free' ? (
                    <NextLink href={PROTOTYPE_REMINDER_ITEM_HREF} className={styles.row}>
                      {rowBody}
                    </NextLink>
                  ) : (
                    <NextLink
                      href="#"
                      title={STUB_TITLES[reminder.kind]}
                      className={styles.row}
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
        )}
      </section>
    </div>
  );
}
