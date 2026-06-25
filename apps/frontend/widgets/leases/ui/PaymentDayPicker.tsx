'use client';

import type { JSX, KeyboardEvent } from 'react';
import { useId, useRef, useState } from 'react';
import {
  Drawer,
  DrawerBackdrop,
  DrawerBody,
  DrawerCloseTrigger,
  DrawerContent,
  DrawerDialog,
  DrawerHeader,
  DrawerHeading,
} from '@heroui/react/drawer';
import { Button } from '@/shared/ui/button';
import styles from './PaymentDayPicker.module.css';

const DAYS: ReadonlyArray<number> = Array.from({ length: 31 }, (_, i) => i + 1);

export type PaymentDayPickerProps = {
  readonly value?: number;
  readonly onChange: (day: number) => void;
};

export function PaymentDayPicker({ value, onChange }: PaymentDayPickerProps): JSX.Element {
  const [isOpen, setIsOpen] = useState(false);
  const triggerId = useId();
  const gridRef = useRef<HTMLDivElement>(null);

  const handleSelect = (day: number) => {
    onChange(day);
    setIsOpen(false);
  };

  const moveFocus = (nextDay: number) => {
    const target = gridRef.current?.querySelector<HTMLButtonElement>(`[data-day="${nextDay}"]`);
    target?.focus();
  };

  const handleDayKeyDown = (event: KeyboardEvent<HTMLButtonElement>, day: number) => {
    const index = day - 1;
    let nextIndex = index;

    switch (event.key) {
      case 'ArrowLeft':
        nextIndex = Math.max(0, index - 1);
        break;
      case 'ArrowRight':
        nextIndex = Math.min(DAYS.length - 1, index + 1);
        break;
      case 'ArrowUp':
        nextIndex = Math.max(0, index - 7);
        break;
      case 'ArrowDown':
        nextIndex = Math.min(DAYS.length - 1, index + 7);
        break;
      case 'Home':
        nextIndex = 0;
        break;
      case 'End':
        nextIndex = DAYS.length - 1;
        break;
      default:
        return;
    }

    event.preventDefault();
    const nextDay = DAYS[nextIndex];
    onChange(nextDay);
    moveFocus(nextDay);
  };

  return (
    <div className={styles.root}>
      <label htmlFor={triggerId} className={styles.label}>
        День оплаты <span className={styles.required}>*</span>
      </label>
      <Button
        id={triggerId}
        type="button"
        variant="secondary"
        size="large"
        fullWidth
        aria-haspopup="dialog"
        aria-expanded={isOpen}
        onClick={() => setIsOpen(true)}
        className={styles.trigger}
      >
        {value ? `${value}-е число` : 'Выбрать'}
      </Button>
      <Drawer isOpen={isOpen} onOpenChange={setIsOpen}>
        <DrawerBackdrop />
        <DrawerContent placement="bottom">
          <DrawerDialog>
            <DrawerCloseTrigger aria-label="Закрыть" />
            <DrawerHeader className={styles.drawerHeader}>
              <DrawerHeading>День оплаты</DrawerHeading>
            </DrawerHeader>
            <DrawerBody className={styles.drawerBody}>
              <div
                ref={gridRef}
                className={styles.grid}
                role="radiogroup"
                aria-label="День оплаты"
              >
                {DAYS.map((day) => {
                  const selected = value === day;
                  const tabbable = selected || (value === undefined && day === 1);

                  return (
                    <button
                      key={day}
                      type="button"
                      role="radio"
                      data-day={day}
                      aria-checked={selected}
                      tabIndex={tabbable ? 0 : -1}
                      className={`${styles.day} ${selected ? styles.dayActive : ''}`}
                      onClick={() => handleSelect(day)}
                      onKeyDown={(event) => handleDayKeyDown(event, day)}
                    >
                      {day}
                    </button>
                  );
                })}
              </div>
            </DrawerBody>
          </DrawerDialog>
        </DrawerContent>
      </Drawer>
    </div>
  );
}
