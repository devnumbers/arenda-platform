// PROTOTYPE — throwaway, issue #106
'use client';

import { type JSX, useState } from 'react';
import NextLink from 'next/link';
import clsx from 'clsx';
import { ArrowLeft, ArrowRight, Plus } from '@/shared/assets/icons';
import { Button } from '@/shared/ui/button';
import { IconButton } from '@/shared/ui/icon-button';
import { LinkButton } from '@/shared/ui/link-button';
import { PageHeader } from '@/shared/ui/page-header';
import { PROTOTYPE_REMINDER_CREATE_HREF, PROTOTYPE_REMINDER_ITEM_HREF } from '../../model/variant';
import {
  formatPrototypeDate,
  formatPrototypeDateWithWeekday,
  prototypeCalendarKindLabels,
  prototypeCalendarStatusLabels,
  prototypeMonthGridWeeks,
  prototypeMonthTitle,
  prototypePropertyDisplayName,
  prototypeReminderIsRecurring,
  prototypeReminderStatus,
  PROTOTYPE_TODAY,
  PROTOTYPE_WEEKDAY_SHORTS,
  remindersForDate,
  type PrototypeCalendarKind,
  type PrototypeCalendarReminder,
} from '../../model/mocks';
import styles from './VariantAMonthGrid.module.css';

export const variantName = 'Месяц-сетка';

type KindFilter = 'all' | PrototypeCalendarKind;

const FILTER_OPTIONS: readonly { readonly value: KindFilter; readonly label: string }[] = [
  { value: 'all', label: 'Все' },
  { value: 'free', label: 'Свободные' },
  { value: 'operation', label: 'По операциям' },
  { value: 'system', label: 'Системные' },
];

const STUB_TITLES: Record<'operation' | 'system', string> = {
  operation: 'Страница операции — вне прототипа',
  system: 'Страница аренды — вне прототипа',
};

function todayCursor(): { readonly year: number; readonly monthIndex: number } {
  const today = new Date(`${PROTOTYPE_TODAY}T00:00:00`);
  return { year: today.getFullYear(), monthIndex: today.getMonth() };
}

function chipClass(reminder: PrototypeCalendarReminder): string {
  return clsx(
    styles.chip,
    reminder.kind === 'free' && styles.chipFree,
    reminder.kind === 'operation' && reminder.operationStatus === 'due_soon' && styles.chipDueSoon,
    reminder.kind === 'operation' && reminder.operationStatus === 'overdue' && styles.chipOverdue,
    reminder.kind === 'system' && styles.chipSystem,
    reminder.propertyId === null && styles.chipOrphan,
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

export function VariantAMonthGrid(): JSX.Element {
  const [cursor, setCursor] = useState(todayCursor);
  const [filter, setFilter] = useState<KindFilter>('all');
  const [selectedDate, setSelectedDate] = useState<string>(PROTOTYPE_TODAY);

  const matchesFilter = (reminder: PrototypeCalendarReminder): boolean =>
    filter === 'all' || reminder.kind === filter;

  const weeks = prototypeMonthGridWeeks(cursor.year, cursor.monthIndex);
  const selectedReminders = remindersForDate(selectedDate).filter(matchesFilter);

  const shiftMonth = (delta: number) => {
    setCursor((prev) => {
      const total = prev.year * 12 + prev.monthIndex + delta;
      return { year: Math.floor(total / 12), monthIndex: ((total % 12) + 12) % 12 };
    });
  };

  const goToday = () => {
    setCursor(todayCursor());
    setSelectedDate(PROTOTYPE_TODAY);
  };

  const handleStubClick = (reminder: PrototypeCalendarReminder) => {
    console.log('[prototype#106] variant A — переход вне прототипа', reminder.kind, reminder.id);
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
          <Button variant="secondary" size="small" onClick={goToday}>
            Сегодня
          </Button>
        </div>
        <div className={styles.filters} role="group" aria-label="Фильтр по типу напоминаний">
          {FILTER_OPTIONS.map((option) => (
            <button
              key={option.value}
              type="button"
              className={clsx(
                styles.filterChip,
                filter === option.value && styles.filterChipActive,
              )}
              onClick={() => setFilter(option.value)}
            >
              {option.label}
            </button>
          ))}
        </div>
      </div>

      <div className={styles.grid}>
        {PROTOTYPE_WEEKDAY_SHORTS.map((weekday) => (
          <div key={weekday} className={styles.weekday}>
            {weekday}
          </div>
        ))}
        {weeks.flat().map((cell) => {
          const dayReminders = remindersForDate(cell.date).filter(matchesFilter);
          const visible = dayReminders.slice(0, 3);
          const hiddenCount = dayReminders.length - visible.length;
          return (
            <button
              key={cell.date}
              type="button"
              className={clsx(
                styles.cell,
                !cell.inMonth && styles.cellNeighbor,
                cell.date === selectedDate && styles.cellSelected,
              )}
              aria-label={formatPrototypeDate(cell.date)}
              onClick={() => setSelectedDate(cell.date)}
            >
              <span className={clsx(styles.dayNumber, cell.isToday && styles.dayNumberToday)}>
                {cell.day}
              </span>
              {visible.map((reminder) => (
                <span key={reminder.id} className={chipClass(reminder)}>
                  {reminder.kind === 'free' && <span className={styles.chipDot} aria-hidden="true" />}
                  {prototypeReminderIsRecurring(reminder) && (
                    <span className={styles.chipRepeat} aria-hidden="true">
                      ↻
                    </span>
                  )}
                  <span className={styles.chipTitle}>{reminder.title}</span>
                </span>
              ))}
              {hiddenCount > 0 && <span className={styles.more}>+{hiddenCount} ещё</span>}
            </button>
          );
        })}
      </div>

      <section className={styles.dayPanel} aria-label="Напоминания выбранного дня">
        <h2 className={styles.dayPanelTitle}>{formatPrototypeDateWithWeekday(selectedDate)}</h2>
        {selectedReminders.length === 0 ? (
          <p className={styles.dayPanelEmpty}>Нет напоминаний</p>
        ) : (
          <ul className={styles.dayList}>
            {selectedReminders.map((reminder) => {
              const status = prototypeReminderStatus(reminder);
              const rowBody = (
                <>
                  <span className={styles.rowTime}>{reminder.time ?? '—'}</span>
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
