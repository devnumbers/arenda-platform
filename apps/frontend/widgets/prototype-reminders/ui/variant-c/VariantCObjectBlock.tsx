// PROTOTYPE — throwaway, issue #105

import type { JSX } from 'react';
import NextLink from 'next/link';
import { ArrowRight, Plus } from '@/shared/assets/icons';
import { Icon } from '@/shared/ui/icon';
import { LinkButton } from '@/shared/ui/link-button';
import { PropertyDetailSection } from '@/widgets/property-detail/ui/PropertyDetailSection';
import { prototypeHref, PROTOTYPE_ROUTES, type PrototypeVariant } from '../../model/variant';
import {
  formatPrototypeDate,
  formatPrototypeDateTime,
  prototypeFrequencyLabels,
  prototypeObjectReminders,
  PROTOTYPE_OBJECT_ID,
} from '../../model/mocks';
import styles from './VariantCObjectBlock.module.css';

export const variantName = 'Форма с живой сводкой';

export type VariantCObjectBlockProps = {
  readonly variant: PrototypeVariant;
};

export function VariantCObjectBlock({ variant }: VariantCObjectBlockProps): JSX.Element {
  const [nearest, ...rest] = prototypeObjectReminders;
  const itemHref = prototypeHref(PROTOTYPE_ROUTES.item, variant);
  const createHref = prototypeHref(PROTOTYPE_ROUTES.create, variant, {
    object: PROTOTYPE_OBJECT_ID,
  });

  return (
    <PropertyDetailSection>
      <h2 className={styles.title}>Напоминания</h2>

      {nearest && (
        <NextLink href={itemHref} className={styles.heroCard}>
          <span className={styles.heroBadge}>{prototypeFrequencyLabels[nearest.frequency]}</span>
          <span className={styles.heroWhen}>
            {formatPrototypeDate(nearest.date)} в {nearest.time}
          </span>
          <span className={styles.heroTitle}>{nearest.title}</span>
        </NextLink>
      )}

      {rest.length > 0 && (
        <ul className={styles.restList}>
          {rest.map((reminder) => (
            <li key={reminder.id}>
              <NextLink href={itemHref} className={styles.restRow}>
                <span className={styles.restName}>{reminder.title}</span>
                <span className={styles.restMeta}>{formatPrototypeDateTime(reminder)}</span>
              </NextLink>
            </li>
          ))}
        </ul>
      )}

      <div className={styles.actions}>
        <LinkButton href={createHref} variant="primary" size="large" fullWidth leftIcon={<Plus />}>
          Создать напоминание
        </LinkButton>
        {/* Календарь — соседний прототип (#106) */}
        <NextLink
          href={prototypeHref('/prototype/calendar', variant)}
          className={styles.allLink}
          title="Календарь — прототип #106"
        >
          Все в календаре
          <Icon size="xs">
            <ArrowRight />
          </Icon>
        </NextLink>
      </div>
    </PropertyDetailSection>
  );
}
