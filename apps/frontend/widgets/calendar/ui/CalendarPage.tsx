'use client';

import { type JSX, useMemo, useState } from 'react';
import clsx from 'clsx';
import { ArrowLeft, ArrowRight, Bell } from '@/shared/assets/icons';
import { Button } from '@/shared/ui/button';
import { EmptyState } from '@/shared/ui/empty-state';
import { IconButton } from '@/shared/ui/icon-button';
import { PageHeader } from '@/shared/ui/page-header';
import { useCalendarReminders } from '@/features/reminders';
import {
  addDays,
  formatDateWithWeekday,
  localDateOf,
  todayISO,
  weekdayShort,
  weekDates,
} from '@/entities/calendar';
import {
  calendarEntryStatusLabels,
  calendarEntryTypeLabels,
  mapCalendarEntries,
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
  return clsx(
    styles.dot,
    entry.eventType === 'operation_due' && styles.dotDueSoon,
    entry.eventType === 'operation_overdue' && styles.dotOverdue,
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

// Сортировка повестки: стабильная по id — все записи являются событиями дня.
function compareEntries(a: CalendarEntry, b: CalendarEntry): number {
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
    () => mapCalendarEntries(data?.items ?? []),
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
      <PageHeader title="Календарь" />

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
              />
            ) : (
              <ul className={styles.agendaList}>
                {selectedReminders.map((reminder, index) => (
                  <li key={`${reminder.id}-${index}`}>
                    <div
                      className={clsx(styles.row, styles.rowStub)}
                      title={STUB_TITLES[reminder.type]}
                    >
                      <span className={styles.rowTime}>весь день</span>
                      <span
                        className={clsx(
                          styles.kindBadge,
                          reminder.type === 'operation' &&
                            styles.kindBadgeOperation,
                          reminder.type === 'system' && styles.kindBadgeSystem,
                        )}
                      >
                        {calendarEntryTypeLabels[reminder.type]}
                      </span>
                      <span className={styles.rowMain}>
                        <span className={styles.rowTitle}>
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
                      {reminder.eventType && (
                        <span className={statusChipClass(reminder.eventType)}>
                          {calendarEntryStatusLabels[reminder.eventType]}
                        </span>
                      )}
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </section>
        </>
      )}
    </div>
  );
}
