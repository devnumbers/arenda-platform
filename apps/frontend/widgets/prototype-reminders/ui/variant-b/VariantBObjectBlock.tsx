// PROTOTYPE — throwaway, issue #105
'use client';

import { type JSX, useState } from 'react';
import NextLink from 'next/link';
import { useRouter } from 'next/navigation';
import clsx from 'clsx';
import { ChevronDown, ChevronUp, Plus } from '@/shared/assets/icons';
import { IconButton } from '@/shared/ui/icon-button';
import { PropertyDetailSection } from '@/widgets/property-detail/ui/PropertyDetailSection';
import { prototypeHref, PROTOTYPE_ROUTES, type PrototypeVariant } from '../../model/variant';
import {
  formatPrototypeDateTime,
  prototypeFrequencyLabels,
  prototypeObjectReminders,
  PROTOTYPE_OBJECT_ID,
} from '../../model/mocks';
import styles from './VariantBObjectBlock.module.css';

export const variantName = 'Визард';

export type VariantBObjectBlockProps = {
  readonly variant: PrototypeVariant;
};

function pluralizeReminders(count: number): string {
  if (count === 1) {
    return 'напоминание';
  }
  if (count >= 2 && count <= 4) {
    return 'напоминания';
  }
  return 'напоминаний';
}

export function VariantBObjectBlock({ variant }: VariantBObjectBlockProps): JSX.Element {
  const router = useRouter();
  const [isExpanded, setIsExpanded] = useState(false);

  const reminders = prototypeObjectReminders;
  const nearest = reminders[0];
  const createHref = prototypeHref(PROTOTYPE_ROUTES.create, variant, {
    object: PROTOTYPE_OBJECT_ID,
  });

  return (
    <PropertyDetailSection>
      <div className={styles.headerRow}>
        <h2 className={styles.title}>Напоминания</h2>
        <div className={styles.headerActions}>
          {/* Календарь — соседний прототип (#106) */}
          <NextLink
            href={prototypeHref('/prototype/calendar', variant)}
            className={styles.calendarLink}
            title="Календарь — прототип #106"
          >
            Календарь
          </NextLink>
          <IconButton
            variant="secondary"
            size="small"
            icon={<Plus />}
            aria-label="Создать напоминание"
            onClick={() => router.push(createHref)}
          />
        </div>
      </div>

      <button
        type="button"
        className={styles.collapsedCard}
        aria-expanded={isExpanded}
        onClick={() => setIsExpanded((prev) => !prev)}
      >
        <span className={styles.summary}>
          {reminders.length} {pluralizeReminders(reminders.length)}
          {nearest && (
            <span className={styles.summaryNearest}>
              · ближайшее: {formatPrototypeDateTime(nearest)}
            </span>
          )}
        </span>
        <span className={styles.chevron} aria-hidden="true">
          {isExpanded ? <ChevronUp /> : <ChevronDown />}
        </span>
      </button>

      <div className={clsx(styles.expandable, isExpanded && styles.expandableOpen)}>
        {isExpanded && (
          <ul className={styles.list}>
            {reminders.map((reminder) => (
              <li key={reminder.id}>
                <NextLink
                  href={prototypeHref(PROTOTYPE_ROUTES.item, variant)}
                  className={styles.row}
                >
                  <span className={styles.rowMain}>
                    <span className={styles.rowName}>{reminder.title}</span>
                    <span className={styles.rowMeta}>{formatPrototypeDateTime(reminder)}</span>
                  </span>
                  <span className={styles.badge}>
                    {prototypeFrequencyLabels[reminder.frequency]}
                  </span>
                </NextLink>
              </li>
            ))}
          </ul>
        )}
      </div>
    </PropertyDetailSection>
  );
}
