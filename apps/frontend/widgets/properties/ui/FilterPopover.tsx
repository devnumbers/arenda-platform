'use client';

import type { JSX, ReactNode } from 'react';
import {
  Popover,
  PopoverContent,
  PopoverDialog,
  PopoverHeading,
  PopoverTrigger,
} from '@heroui/react/popover';
import { Button as HeroButton } from '@heroui/react/button';
import { ChevronDown } from '@/shared/assets/icons';
import { Icon } from '@/shared/ui/icon';
import styles from './FilterPopover.module.css';

export type FilterPopoverProps = {
  readonly label: string;
  readonly activeCount: number;
  readonly children: ReactNode;
};

export function FilterPopover({ label, activeCount, children }: FilterPopoverProps): JSX.Element {
  const triggerLabel = activeCount > 0 ? `${label} ${activeCount}` : label;

  return (
    <Popover>
      <PopoverTrigger>
        <HeroButton
          className={styles.trigger}
          variant="secondary"
          size="sm"
        >
          {triggerLabel}
          <Icon size="xs">
            <ChevronDown />
          </Icon>
        </HeroButton>
      </PopoverTrigger>
      <PopoverContent className={styles.content}>
        <PopoverDialog>
          <PopoverHeading className={styles.heading}>{label}</PopoverHeading>
          {children}
        </PopoverDialog>
      </PopoverContent>
    </Popover>
  );
}
