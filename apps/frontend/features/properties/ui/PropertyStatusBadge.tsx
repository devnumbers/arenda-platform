import type { JSX } from 'react';
import clsx from 'clsx';
import { CalendarSmall, KeySmall, PaintBrushSmall } from '@/shared/assets/icons';
import type { PropertyBadge, PropertyBadgeTone } from '@/features/properties/lib/property-badges';
import styles from './PropertyStatusBadge.module.css';

/**
 * Бейдж-пилюля карточки объекта — канон Figma StatusHouseBadge (1603:91041,
 * пересобран по резолюции #584 в тикете #586): иконка S 16 + текст 14/16
 * Medium. Тоны: accent — белая пилюля с голубым текстом (занятость),
 * accent-filled — залитая синим «Аренда завершена», warning — белая с
 * жёлтым «на ремонте» (цвет макета #FFB900). Иконка по смыслу бейджа:
 * календарь (осталось N / аренда с), ключ (период подошёл к концу),
 * кисть (ремонт).
 */
const toneClass: Record<PropertyBadgeTone, string> = {
  accent: styles.accent ?? '',
  'accent-filled': styles.accentFilled ?? '',
  warning: styles.warning ?? '',
};

const badgeIcon: Record<PropertyBadge['key'], JSX.Element> = {
  'rental-months': <CalendarSmall />,
  'rental-completed': <KeySmall />,
  'rental-upcoming': <CalendarSmall />,
  maintenance: <PaintBrushSmall />,
};

export function PropertyStatusBadge({ badge }: { readonly badge: PropertyBadge }): JSX.Element {
  return (
    <span className={clsx(styles.badge, toneClass[badge.tone])}>
      <span className={styles.icon} aria-hidden>
        {badgeIcon[badge.key]}
      </span>
      {badge.label}
    </span>
  );
}
