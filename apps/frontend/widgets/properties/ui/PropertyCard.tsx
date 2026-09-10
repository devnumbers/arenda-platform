'use client';

import type { JSX } from 'react';
import NextLink from 'next/link';
import { ROUTES } from '@/shared/config/routes';
import { PinSmall, SmallArrowRight } from '@/shared/assets/icons';
import type { IsoDate } from '@/shared/lib/calendar';
import {
  hasPropertyAttentionDot,
  propertyBadges,
  PropertyStatusBadge,
  type PropertyBadge,
} from '@/features/properties';
import { AccessRoleBadge } from '@/entities/access';
import { PropertyAvatar, type Property } from '@/entities/property';
import styles from './PropertyCard.module.css';

/**
 * Карточка объекта в списке-хабе (Figma 1590:88756 — мобайл, 1603:88972 —
 * планшет; тикет #586): серая карточка radius 24, круг-фото 64 (мобайл ≤560
 * над текстом, 561+ — слева вровень), имя 20/24 до трёх строк, адрес с
 * скрепкой у основного объекта (Icon/S/Pin), красная точка на круге
 * (резолюция #584), бейдж роли доступа у чужого объекта, арендный бейдж +
 * «на ремонте», шеврон у правого верхнего края. Вся карточка — ссылка на
 * объект (оверлей). «Сегодня владельца» (ADR 0048) нужно только бейджу
 * «Осталось N месяцев» — остальные бейджи рендерятся, не дожидаясь его.
 */
export type PropertyCardProps = {
  readonly property: Property;
  /** «Сегодня владельца» (ADR 0048) — бейдж «Осталось N месяцев». */
  readonly today?: IsoDate;
};

export function PropertyCard({ property, today }: PropertyCardProps): JSX.Element {
  const isPrimary = property.pinned_at !== null;
  const badges: readonly PropertyBadge[] = propertyBadges(property, today);
  const accessRole = property.access && property.access.role !== 'owner'
    ? property.access.role
    : null;

  return (
    <article className={styles.root}>
      <NextLink
        href={ROUTES.property(property.id)}
        className={styles.cardLink}
        aria-label={`Открыть объект ${property.name}`}
      />
      <div className={styles.body}>
        <PropertyAvatar
          photoUrl={property.photos?.[0]?.url ?? null}
          withAttentionDot={hasPropertyAttentionDot(property)}
        />
        <div className={styles.info}>
          <h3 className={styles.title}>{property.name}</h3>
          <p className={styles.address}>
            {isPrimary && <PinSmall className={styles.pin} aria-hidden />}
            <span className={styles.addressText}>{property.address}</span>
          </p>
          {(badges.length > 0 || accessRole !== null) && (
            <div className={styles.badges}>
              {accessRole !== null && <AccessRoleBadge role={accessRole} />}
              {badges.map((badge) => (
                <PropertyStatusBadge key={badge.key} badge={badge} />
              ))}
            </div>
          )}
        </div>
      </div>
      <SmallArrowRight className={styles.chevron} aria-hidden />
    </article>
  );
}
