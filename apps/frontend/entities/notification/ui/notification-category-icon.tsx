'use client';

import type { JSX } from 'react';
import Image from 'next/image';
import { BoldKey, NotificationDot, StatusIconDanger } from '@/shared/assets/icons';
import type { NotificationCategory } from '../model/types';

/**
 * Иконка категории строки ленты (#744, Figma 651:6743 Category Icon
 * + 2329:149013): круг 44 с белым кольцом 2.5, внутри 3D-иллюстрация
 * категории 28 (PNG-эталоны из макета, канон §10 — не генерируются) или
 * канонный BoldKey у платежей (в макете 2329:149014 платежи — белая
 * bold-иконка на синем, 3D для категории нет); красная точка непрочитанного
 * — канон NotificationDot в левом-верхнем углу (как у PropertyAvatar).
 * У Тарифа — служебный бейдж опасности справа-снизу, наполовину вне круга
 * (Figma 2329:149016, Icon/Color/DangerWhite): белый кант иконки красится
 * через text-surface (stroke="currentColor" в SVG — канон StatusIcon),
 * отделяя бейдж от тёмной 3D-иллюстрации.
 *
 * PNG рендерятся unoptimized: next/image сжимает мелкие иллюстрации
 * (w=64&q=75) в мыло — решение владельца 18.09.2026; эталоны экспортированы
 * @4x (112×112, потолок Figma API scale=4).
 */

/** Светло-голубая подложка 3D-категорий (Figma color-light/blue, #E9F2FF —
 * в токенах значения нет; канон цвета не расширял). */
const CATEGORY_LIGHT_BG = 'bg-[#e9f2ff]';

type CategoryVisual =
  | { readonly kind: 'image'; readonly src: string; readonly className: string }
  | { readonly kind: 'boldKey'; readonly className: string };

const CATEGORY_VISUALS: Record<NotificationCategory, CategoryVisual> = {
  rental: { kind: 'image', src: '/images/notifications/category-rental.png', className: CATEGORY_LIGHT_BG },
  payments_operations: { kind: 'boldKey', className: 'bg-primary' },
  tasks: { kind: 'image', src: '/images/notifications/category-tasks.png', className: CATEGORY_LIGHT_BG },
  shared_access: {
    kind: 'image',
    src: '/images/notifications/category-shared-access.png',
    className: CATEGORY_LIGHT_BG,
  },
  tariff: { kind: 'image', src: '/images/notifications/category-tariff.png', className: CATEGORY_LIGHT_BG },
  system: { kind: 'image', src: '/images/notifications/category-system.png', className: 'bg-surface-muted' },
};

const BADGE_CATEGORIES: ReadonlySet<NotificationCategory> = new Set(['tariff']);

export function NotificationCategoryIcon({
  category,
  unread,
}: {
  readonly category: NotificationCategory;
  readonly unread: boolean;
}): JSX.Element {
  const visual = CATEGORY_VISUALS[category];
  return (
    <div className="relative h-11 w-11 shrink-0" aria-hidden>
      <div
        className={`flex h-full w-full items-center justify-center rounded-pill ring-[2.5px] ring-white ${visual.className}`}
      >
        {visual.kind === 'image' ? (
          <Image src={visual.src} alt="" width={28} height={28} className="h-7 w-7" unoptimized />
        ) : (
          <BoldKey className="h-6 w-6 text-white" />
        )}
      </div>
      {unread && <NotificationDot className="absolute left-0 top-0 h-3.5 w-3.5" />}
      {BADGE_CATEGORIES.has(category) && (
        // Кант бейджа — stroke="currentColor" в самом SVG, красится под
        // поверхность ленты (прецедент features/payment-categories
        // category-icon): белая лента → text-surface.
        <StatusIconDanger
          className="absolute -bottom-2 -right-2 h-6 w-6 text-surface"
          aria-hidden
        />
      )}
    </div>
  );
}
