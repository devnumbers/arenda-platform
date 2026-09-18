'use client';

import type { JSX } from 'react';
import NextLink from 'next/link';
import { ROUTES } from '@/shared/config/routes';
import { BoldUser, PinSmall, SmallArrowRight } from '@/shared/assets/icons';
import type { IsoDate } from '@/shared/lib/calendar';
import { cardParticipantNames } from '../lib/property-participants';
import {
  archivedPropertyBadge,
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
 * планшет; тикет #586) и в архиве (вариант archived, Figma 1603:92102,
 * тикет #587): серая карточка radius 24, круг-фото 64 (мобайл ≤560 над
 * текстом, 561+ — слева вровень), имя 20/24 до трёх строк, адрес со
 * скрепкой у основного объекта (Icon/S/Pin), красная точка на круге
 * (резолюция #584), бейдж роли доступа у чужого объекта, арендный бейдж +
 * «на ремонте», шеврон у правого верхнего края. Вся карточка — ссылка на
 * объект (оверлей). «Сегодня владельца» (ADR 0048) нужно только бейджу
 * «Осталось N месяцев» — остальные бейджи рендерятся, не дожидаясь его.
 * Под бейджами — ряд участников своих объектов (#702, Figma 2200-97368):
 * круг 24 с BoldUser и имя 13/500 на участника, в порядке приглашения.
 *
 * Вариант archived — карточка архива: та же анатомия, но смысловая
 * начинка архивная — единственный бейдж «В архиве», без точки, скрепки
 * и бейджа роли (макет 1603:92102).
 */
export type PropertyCardProps = {
  readonly property: Property;
  /** «Сегодня владельца» (ADR 0048) — бейдж «Осталось N месяцев». */
  readonly today?: IsoDate;
  /** archived — карточка экрана «Архивные объекты» (#587). */
  readonly variant?: 'list' | 'archived';
  /** nonInteractive — без оверлея-ссылки: карточка-подвесший (#702)
   * рендерит обычную анатомию под блюром, тап ведёт в шит причины, а не на
   * деталь объекта. */
  readonly nonInteractive?: boolean;
};

export function PropertyCard({ property, today, variant = 'list', nonInteractive = false }: PropertyCardProps): JSX.Element {
  const isArchived = variant === 'archived';
  const isPrimary = !isArchived && property.pinned_at !== null;
  const badges: readonly PropertyBadge[] = isArchived
    ? [archivedPropertyBadge]
    : propertyBadges(property, today);
  const accessRole = !isArchived && property.access && property.access.role !== 'owner'
    ? property.access.role
    : null;
  // Ряд участников (#702): активные участники своих объектов в порядке
  // приглашения; у архивной карточки ряд тоже рисуется (Figma 2213-98944).
  const participantNames = cardParticipantNames(property);

  return (
    <article className={styles.root}>
      {!nonInteractive && (
        <NextLink
          href={ROUTES.property(property.id)}
          className={styles.cardLink}
          aria-label={`Открыть объект ${property.name}`}
        />
      )}
      <div className={styles.body}>
        <PropertyAvatar
          photoUrl={property.photos?.[0]?.url ?? null}
          withAttentionDot={!isArchived && hasPropertyAttentionDot(property)}
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
          {participantNames.length > 0 && (
            <ul className={styles.participants} data-testid="property-participants">
              {participantNames.map((name, index) => (
                <li key={`${name}-${index}`} className={styles.participant}>
                  <span className={styles.participantAvatar} aria-hidden>
                    <BoldUser className={styles.participantGlyph} />
                  </span>
                  <span className={styles.participantName}>{name}</span>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
      <SmallArrowRight className={styles.chevron} aria-hidden />
    </article>
  );
}
