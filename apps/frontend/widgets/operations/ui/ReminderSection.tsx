'use client';

import { useId, type JSX } from 'react';
import clsx from 'clsx';
import { Checkbox } from '@heroui/react';
import styles from './ReminderSection.module.css';

export type ReminderOffsetDays = 1 | 3 | 7;

type ReminderSectionProps = {
  readonly enabled: boolean;
  readonly offsetDays: ReminderOffsetDays;
  readonly onEnabledChange: (enabled: boolean) => void;
  readonly onOffsetChange: (offsetDays: ReminderOffsetDays) => void;
  readonly disabled?: boolean;
};

const REMINDER_OFFSET_OPTIONS: {
  readonly value: ReminderOffsetDays;
  readonly label: string;
}[] = [
  { value: 1, label: 'За 1 день' },
  { value: 3, label: 'За 3 дня' },
  { value: 7, label: 'За 7 дней' },
];

export function ReminderSection({
  enabled,
  offsetDays,
  onEnabledChange,
  onOffsetChange,
  disabled,
}: ReminderSectionProps): JSX.Element {
  const groupId = useId();

  return (
    <div className={styles.reminder}>
      <Checkbox
        isSelected={enabled}
        onChange={onEnabledChange}
        isDisabled={disabled}
        className={styles.checkbox}
      >
        <Checkbox.Content className={styles.checkboxContent}>
          <Checkbox.Control className={styles.checkboxControl}>
            <Checkbox.Indicator className={styles.checkboxIndicator} />
          </Checkbox.Control>
          <span>Добавить SMS-напоминание</span>
        </Checkbox.Content>
      </Checkbox>

      {enabled && (
        <div
          className={styles.offsetGroup}
          role="radiogroup"
          aria-labelledby={`${groupId}-label`}
        >
          <span id={`${groupId}-label`} className={styles.offsetLabel}>
            За сколько дней напомнить
          </span>
          <div className={styles.offsetOptions}>
            {REMINDER_OFFSET_OPTIONS.map((option) => {
              const isSelected = offsetDays === option.value;
              return (
                <button
                  key={option.value}
                  type="button"
                  role="radio"
                  aria-checked={isSelected}
                  disabled={disabled}
                  className={clsx(
                    styles.offsetOption,
                    isSelected && styles.selected,
                  )}
                  onClick={() => onOffsetChange(option.value)}
                >
                  <span className={styles.radio} aria-hidden="true">
                    <span className={styles.radioDot} />
                  </span>
                  {option.label}
                </button>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}
