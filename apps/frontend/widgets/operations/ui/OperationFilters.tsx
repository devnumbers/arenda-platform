'use client';

import type { JSX } from 'react';
import { Button } from '@/shared/ui/button';
import {
  formatDateForApi,
  startOfMonth,
  endOfMonth,
} from '@/entities/operation/lib/dates';
import type { OperationKind } from './OperationsPage';
import styles from './OperationFilters.module.css';

export type OperationFiltersState = {
  readonly from: string;
  readonly to: string;
  readonly status: ReadonlyArray<string>;
  readonly kind: OperationKind;
};

export type OperationFiltersProps = {
  readonly filters: OperationFiltersState;
  readonly onChange: (filters: OperationFiltersState) => void;
  readonly onKindChange: (kind: OperationKind) => void;
};

type Period = 'month' | 'quarter' | 'year';

const COMPLETED_STATUSES: ReadonlyArray<string> = ['paid', 'received'];

function getCurrentMonthRange(): { from: string; to: string } {
  const now = new Date();
  return {
    from: formatDateForApi(startOfMonth(now)),
    to: formatDateForApi(endOfMonth(now)),
  };
}

function getQuarterRange(date: Date): { from: string; to: string } {
  const quarter = Math.floor(date.getMonth() / 3);
  const from = new Date(date.getFullYear(), quarter * 3, 1);
  const to = new Date(date.getFullYear(), quarter * 3 + 3, 0);
  return { from: formatDateForApi(from), to: formatDateForApi(to) };
}

function getYearRange(date: Date): { from: string; to: string } {
  const from = new Date(date.getFullYear(), 0, 1);
  const to = new Date(date.getFullYear(), 11, 31);
  return { from: formatDateForApi(from), to: formatDateForApi(to) };
}

function setPeriod(
  period: Period,
  onChange: (filters: OperationFiltersState) => void,
  currentFilters: OperationFiltersState,
): void {
  const now = new Date();
  const range =
    period === 'month'
      ? getCurrentMonthRange()
      : period === 'quarter'
        ? getQuarterRange(now)
        : getYearRange(now);
  onChange({ ...currentFilters, from: range.from, to: range.to });
}

function isCompletedSelected(status: ReadonlyArray<string>): boolean {
  return COMPLETED_STATUSES.every((value) => status.includes(value));
}

function toggleCompleted(status: ReadonlyArray<string>): string[] {
  if (isCompletedSelected(status)) {
    return status.filter((value) => !COMPLETED_STATUSES.includes(value));
  }
  const next = new Set(status);
  COMPLETED_STATUSES.forEach((value) => next.add(value));
  return Array.from(next);
}

function toggleStatus(status: ReadonlyArray<string>, value: string): string[] {
  if (status.includes(value)) {
    return status.filter((item) => item !== value);
  }
  return [...status, value];
}

function hasActiveFilters(filters: OperationFiltersState): boolean {
  const currentMonth = getCurrentMonthRange();
  const statusActive = filters.status.length > 0;
  const periodActive =
    filters.from !== currentMonth.from || filters.to !== currentMonth.to;
  const kindActive = filters.kind !== 'all';
  return statusActive || periodActive || kindActive;
}

const STATUS_CHIPS: ReadonlyArray<{
  readonly key: 'all' | 'pending' | 'overdue' | 'completed';
  readonly label: string;
}> = [
  { key: 'all', label: 'Все' },
  { key: 'pending', label: 'Запланирована' },
  { key: 'overdue', label: 'Просрочена' },
  { key: 'completed', label: 'Выполнена' },
];

export function OperationFilters({
  filters,
  onChange,
  onKindChange,
}: OperationFiltersProps): JSX.Element {
  const handleStatusClick = (key: (typeof STATUS_CHIPS)[number]['key']) => {
    if (key === 'all') {
      onChange({ ...filters, status: [] });
      return;
    }

    if (key === 'completed') {
      onChange({ ...filters, status: toggleCompleted(filters.status) });
      return;
    }

    onChange({ ...filters, status: toggleStatus(filters.status, key) });
  };

  const isSelected = (key: (typeof STATUS_CHIPS)[number]['key']): boolean => {
    if (key === 'all') return filters.status.length === 0;
    if (key === 'completed') return isCompletedSelected(filters.status);
    return filters.status.includes(key);
  };

  const handleReset = () => {
    const currentMonth = getCurrentMonthRange();
    onChange({ from: currentMonth.from, to: currentMonth.to, status: [], kind: 'all' });
    onKindChange('all');
  };

  const KIND_CHIPS: ReadonlyArray<{
    readonly key: OperationKind;
    readonly label: string;
  }> = [
    { key: 'all', label: 'Все' },
    { key: 'onetime', label: 'Разовые' },
    { key: 'recurring', label: 'Регулярные' },
  ];

  return (
    <div className={styles.root}>
      <div className={styles.row}>
        <div className={styles.periodGroup}>
          <Button
            variant="icon-black"
            size="small"
            onClick={() => setPeriod('month', onChange, filters)}
          >
            Месяц
          </Button>
          <Button
            variant="icon-black"
            size="small"
            onClick={() => setPeriod('quarter', onChange, filters)}
          >
            Квартал
          </Button>
          <Button
            variant="icon-black"
            size="small"
            onClick={() => setPeriod('year', onChange, filters)}
          >
            Год
          </Button>
        </div>

        {hasActiveFilters(filters) && (
          <Button
            variant="clear"
            size="small"
            onClick={handleReset}
            className={styles.resetButton}
          >
            Сбросить
          </Button>
        )}
      </div>

      <div className={styles.chipGroup} role="group" aria-label="Фильтр по статусу">
        {STATUS_CHIPS.map((chip) => (
          <Button
            key={chip.key}
            variant={isSelected(chip.key) ? 'secondary' : 'icon-black'}
            size="small"
            onClick={() => handleStatusClick(chip.key)}
          >
            {chip.label}
          </Button>
        ))}
      </div>

      <div className={styles.chipGroup} role="group" aria-label="Фильтр по виду">
        {KIND_CHIPS.map((chip) => {
          const isKindSelected = filters.kind === chip.key;
          return (
            <Button
              key={chip.key}
              variant={isKindSelected ? 'secondary' : 'icon-black'}
              size="small"
              onClick={() => onKindChange(chip.key)}
            >
              {chip.label}
            </Button>
          );
        })}
      </div>
    </div>
  );
}
