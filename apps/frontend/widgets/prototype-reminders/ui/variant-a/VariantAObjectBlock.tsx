// PROTOTYPE — throwaway, issue #105

import type { JSX } from 'react';
import NextLink from 'next/link';
import { ArrowRight, Bell, Plus } from '@/shared/assets/icons';
import { Icon } from '@/shared/ui/icon';
import { LinkButton } from '@/shared/ui/link-button';
import { EmptyState } from '@/shared/ui/empty-state';
import { PropertyDetailSection } from '@/widgets/property-detail/ui/PropertyDetailSection';
import { prototypeHref, PROTOTYPE_ROUTES, type PrototypeVariant } from '../../model/variant';
import {
  formatPrototypeDateTime,
  prototypeFrequencyLabels,
  prototypeObjectReminders,
  PROTOTYPE_OBJECT_ID,
} from '../../model/mocks';
import styles from './VariantAObjectBlock.module.css';

export const variantName = 'Одноэкранная форма';

export type VariantAObjectBlockProps = {
  readonly variant: PrototypeVariant;
};

export function VariantAObjectBlock({ variant }: VariantAObjectBlockProps): JSX.Element {
  const reminders = prototypeObjectReminders;
  const createHref = prototypeHref(PROTOTYPE_ROUTES.create, variant, {
    object: PROTOTYPE_OBJECT_ID,
  });

  return (
    <PropertyDetailSection>
      <div className={styles.headerRow}>
        <h2 className={styles.title}>
          Напоминания
          {reminders.length > 0 && <span className={styles.count}> {reminders.length}</span>}
        </h2>
        {/* Календарь — соседний прототип (#106) */}
        <NextLink
          href={prototypeHref('/prototype/calendar', variant)}
          className={styles.calendarLink}
          title="Календарь — прототип #106"
        >
          В календарь
          <Icon size="xs">
            <ArrowRight />
          </Icon>
        </NextLink>
      </div>

      {reminders.length === 0 ? (
        <EmptyState
          icon={<Bell />}
          subtitle="По этому объекту пока нет напоминаний"
          actionHref={createHref}
          actionText="Добавить напоминание"
        />
      ) : (
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
                <span className={styles.badge}>{prototypeFrequencyLabels[reminder.frequency]}</span>
              </NextLink>
            </li>
          ))}
        </ul>
      )}

      <LinkButton
        href={createHref}
        variant="secondary"
        size="medium"
        fullWidth
        leftIcon={<Plus />}
      >
        Добавить напоминание
      </LinkButton>
    </PropertyDetailSection>
  );
}
