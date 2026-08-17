'use client';

import { type JSX, useMemo, useState } from 'react';
import NextLink from 'next/link';
import clsx from 'clsx';
import { ArrowLeft, ArrowRight, Bell, Plus } from '@/shared/assets/icons';
import { Button } from '@/shared/ui/button';
import { EmptyState } from '@/shared/ui/empty-state';
import { IconButton } from '@/shared/ui/icon-button';
import { LinkButton } from '@/shared/ui/link-button';
import { PageHeader } from '@/shared/ui/page-header';
import { useCalendarReminders } from '@/features/reminders';
import { ROUTES } from '@/shared/config/routes';
import {
  addDays,
  formatDateWithWeekday,
  localDateOf,
  localTimeOf,
  todayISO,
  weekdayShort,
  weekDates,
} from '@/entities/calendar';
import {
  calendarEntryStatusLabels,
  calendarEntryTypeLabels,
  entryStatusEventType,
  isRecurring,
  mapCalendarEntry,
  propertyDisplayName,
} from '@/entities/calendar';
import type {
  CalendarEntry,
  CalendarEntryEventType,
} from '@/entities/calendar';
import styles from './CalendarPage.module.css';

// operation/system пока ведут в stub — страницы операции/аренды вне скоупа этого тикета.
const STUB_TITLES: Record<'operation' | 'system', string> = {
  operation: 'Страница операции — скоро',
  system: 'Страница аренды — скоро',
};

function dotClass(entry: CalendarEntry): string {
  const eventType = entryStatusEventType(entry);
  return clsx(
    styles.dot,
    entry.type === 'free' && styles.dotFree,
    eventType === 'operation_due' && styles.dotDueSoon,
    eventType === 'operation_overdue' && styles.dotOverdue,
    entry.type === 'system' && styles.dotSystem,
  );
}

function statusChipClass(eventType: CalendarEntryEventType): string {
  return clsx(
    styles.statusChip,
    eventType === 'operation_overdue' && styles.statusChipOverdue,
    eventType === 'operation_due' && styles.statusChipDueSoon,
    (eventType === 'lease_expiring' || eventType === 'lease_requires_action') &&
      styles.statusChipSystem,
  );
}

// Сортировка повестки: по времени; безвременные (operation/system — события дня) — после временных.
function compareEntries(a: CalendarEntry, b: CalendarEntry): number {
  const aTime = localTimeOf(a.scheduledAt, a.type);
  const bTime = localTimeOf(b.scheduledAt, b.type);
  if (aTime === null && bTime !== null) return 1;
  if (aTime !== null && bTime === null) return -1;
  if (aTime !== null && bTime !== null && aTime !== bTime) {
    return aTime.localeCompare(bTime);
  }
  return a.id.localeCompare(b.id);
}

export function CalendarPage(): JSX.Element {
  const today = todayISO();
  const [weekAnchor, setWeekAnchor] = useState<string>(today);
  const [selectedDate, setSelectedDate] = useState<string>(today);

  const days = useMemo(() => weekDates(weekAnchor), [weekAnchor]);
  const from = days[0];
  const to = addDays(from, 7); // полуоткрытый [from, to) — API требует to > from

  const { data, isLoading, isError } = useCalendarReminders(from, to);

  const entries = useMemo(
    () => (data?.items ?? []).map(mapCalendarEntry),
    [data],
  );

  // Группировка по локальным датам.
  const byDate = useMemo(() => {
    const map = new Map<string, CalendarEntry[]>();
    for (const entry of entries) {
      const date = localDateOf(entry.scheduledAt);
      const bucket = map.get(date);
      if (bucket) {
        bucket.push(entry);
      } else {
        map.set(date, [entry]);
      }
    }
    for (const bucket of map.values()) {
      bucket.sort(compareEntries);
    }
    return map;
  }, [entries]);

  const shiftWeek = (delta: number) => {
    setWeekAnchor((prev) => addDays(prev, delta * 7));
  };

  const goToday = () => {
    setWeekAnchor(today);
    setSelectedDate(today);
  };

  const selectedReminders = byDate.get(selectedDate) ?? [];

  return (
    <div className={styles.root}>
      <PageHeader
        title="Календарь"
        actions={
          <LinkButton
            href={ROUTES.freeReminderNew}
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

      {isLoading && <p className={styles.loading}>Загрузка…</p>}
      {isError && (
        <p className={styles.error}>Не удалось загрузить напоминания.</p>
      )}

      {!isLoading && !isError && (
        <>
          <div className={styles.weekStrip}>
            {days.map((date) => {
              const dayReminders = byDate.get(date) ?? [];
              return (
                <button
                  key={date}
                  type="button"
                  className={clsx(
                    styles.dayCard,
                    date === selectedDate && styles.dayCardSelected,
                    date === today && styles.dayCardToday,
                  )}
                  aria-pressed={date === selectedDate}
                  onClick={() => setSelectedDate(date)}
                >
                  <span className={styles.dayCardWeekday}>
                    {weekdayShort(date)}
                  </span>
                  <span className={styles.dayCardNumber}>
                    {Number(date.slice(8, 10))}
                  </span>
                  <span className={styles.dayCardDots}>
                    {dayReminders.slice(0, 3).map((reminder, index) => (
                      <span
                        key={`${reminder.id}-${index}`}
                        className={dotClass(reminder)}
                        aria-hidden="true"
                      />
                    ))}
                    {dayReminders.length > 0 && (
                      <span className={styles.dayCardCount}>
                        {dayReminders.length}
                      </span>
                    )}
                  </span>
                </button>
              );
            })}
          </div>

          <section
            className={styles.agenda}
            aria-label="Повестка выбранного дня"
          >
            <h2 className={styles.agendaTitle}>
              {formatDateWithWeekday(selectedDate)}
            </h2>
            {selectedReminders.length === 0 ? (
              <EmptyState
                icon={<Bell />}
                title="Нет напоминаний"
                subtitle="На этот день ничего не запланировано"
                actionNode={
                  <LinkButton
                    href={ROUTES.freeReminderNew}
                    variant="primary"
                    size="medium"
                    leftIcon={<Plus />}
                  >
                    Создать напоминание
                  </LinkButton>
                }
              />
            ) : (
              <ul className={styles.agendaList}>
                {selectedReminders.map((reminder, index) => {
                  const eventType = entryStatusEventType(reminder);
                  const time = localTimeOf(reminder.scheduledAt, reminder.type);
                  const rowBody = (
                    <>
                      <span className={styles.rowTime}>
                        {time ?? 'весь день'}
                      </span>
                      <span
                        className={clsx(
                          styles.kindBadge,
                          reminder.type === 'free' && styles.kindBadgeFree,
                          reminder.type === 'operation' &&
                            styles.kindBadgeOperation,
                          reminder.type === 'system' && styles.kindBadgeSystem,
                        )}
                      >
                        {calendarEntryTypeLabels[reminder.type]}
                      </span>
                      <span className={styles.rowMain}>
                        <span className={styles.rowTitle}>
                          {isRecurring(reminder) && (
                            <span
                              className={styles.rowRepeat}
                              aria-hidden="true"
                            >
                              ↻
                            </span>
                          )}
                          {reminder.title}
                        </span>
                        <span
                          className={clsx(
                            styles.rowProperty,
                            !reminder.hasProperty && styles.rowPropertyOrphan,
                          )}
                        >
                          {propertyDisplayName(reminder)}
                        </span>
                      </span>
                      {eventType && (
                        <span className={statusChipClass(eventType)}>
                          {calendarEntryStatusLabels[eventType]}
                        </span>
                      )}
                    </>
                  );

                  if (reminder.type === 'free') {
                    const href = reminder.freeReminderId
                      ? ROUTES.freeReminder(reminder.freeReminderId)
                      : ROUTES.freeReminderNew;
                    return (
                      <li key={`${reminder.id}-${index}`}>
                        <NextLink href={href} className={styles.row}>
                          {rowBody}
                        </NextLink>
                      </li>
                    );
                  }

                  return (
                    <li key={`${reminder.id}-${index}`}>
                      <div
                        className={clsx(styles.row, styles.rowStub)}
                        title={STUB_TITLES[reminder.type]}
                      >
                        {rowBody}
                      </div>
                    </li>
                  );
                })}
              </ul>
            )}
          </section>
        </>
      )}
    </div>
  );
}
