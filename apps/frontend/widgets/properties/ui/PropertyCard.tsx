'use client';

import { useState, type JSX } from 'react';
import NextLink from 'next/link';
import { ROUTES } from '@/shared/config/routes';
import { BoldUser, PinSmall, SmallArrowRight } from '@/shared/assets/icons';
import type { IsoDate } from '@/shared/lib/calendar';
import { cardOwnerName } from '../lib/property-owner-name';
import {
  archivedPropertyBadge,
  hasPropertyAttentionDot,
  propertyBadges,
  PropertyStatusBadge,
  type PropertyBadge,
} from '@/features/properties';
import { PropertyAvatar, type Property } from '@/entities/property';
import styles from './PropertyCard.module.css';

/**
 * Карточка объекта в списке-хабе (Figma 1590:88756 — мобайл, 1603:88972 —
 * планшет; тикет #586) и в архиве (вариант archived, Figma 1603:92102,
 * тикет #587): серая карточка radius 24, круг-фото 64 (мобайл ≤560 над
 * текстом, 561+ — слева вровень), имя 20/24 до трёх строк, адрес со
 * скрепкой у основного объекта (Icon/S/Pin), красная точка на круге
 * (резолюция #584), арендный бейдж + «на ремонте», шеврон у правого
 * верхнего края. Вся карточка — ссылка на объект (оверлей). «Сегодня
 * владельца» (ADR 0048) нужно только бейджу «Осталось N месяцев» —
 * остальные бейджи рендерятся, не дожидаясь его.
 * Под бейджами — ряд владельца ЧУЖОГО объекта (решение владельца по итогам
 * обхода #756): круг 24 с BoldUser и имя 13/500 — карточка показывает, чей
 * это объект и кто выдал доступ; на своих карточках ряда нет. Бейдж роли
 * доступа («Редактирование/Просмотр») на карточках больше не рисуется.
 *
 * Вариант archived — карточка архива: та же анатомия, но смысловая
 * начинка архивная — единственный бейдж «В архиве», без точки и скрепки
 * (макет 1603:92102); ряд владельца чужого архивного объекта — как в
 * основном списке.
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
  // Ряд владельца (решение по итогам обхода #756): только чужие объекты.
  const ownerName = cardOwnerName(property);

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
          photoUrl={property.photoUrl}
          type={property.type}
          withAttentionDot={!isArchived && hasPropertyAttentionDot(property)}
        />
        <div className={styles.info}>
          <h3 className={styles.title}>{property.name}</h3>
          <p className={styles.address}>
            {isPrimary && <PinSmall className={styles.pin} aria-hidden />}
            <span className={styles.addressText}>{property.address}</span>
          </p>
          {badges.length > 0 && (
            <div className={styles.badges}>
              {badges.map((badge) => (
                <PropertyStatusBadge key={badge.key} badge={badge} />
              ))}
            </div>
          )}
          {ownerName !== null && (
            <ul className={styles.participants} data-testid="property-owner">
              <li className={styles.participant}>
                <OwnerChipAvatar photoUrl={property.access?.ownerPhotoUrl} />
                <span className={styles.participantName}>{ownerName}</span>
              </li>
            </ul>
          )}
        </div>
      </div>
      <SmallArrowRight className={styles.chevron} aria-hidden />
    </article>
  );
}

/** Аватар владельца в чипе карточки (решение #1286): фото профиля 24,
 * иначе — и при битом фото (404 стрима) — заглушка BoldUser. */
function OwnerChipAvatar({ photoUrl }: { readonly photoUrl?: string | null }): JSX.Element {
  const [photoBroken, setPhotoBroken] = useState(false);
  const showPhoto = photoUrl != null && photoUrl !== '' && !photoBroken;
  return (
    <span className={styles.participantAvatar} aria-hidden>
      {showPhoto ? (
        <img
          src={photoUrl}
          alt=""
          className={styles.participantAvatarImg}
          onError={() => setPhotoBroken(true)}
        />
      ) : (
        <BoldUser className={styles.participantGlyph} />
      )}
    </span>
  );
}
