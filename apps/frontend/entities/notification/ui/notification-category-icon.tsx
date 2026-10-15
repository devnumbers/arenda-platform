'use client';

import { useState, type JSX } from 'react';
import Image from 'next/image';
import { BoldKey, BoldOther, NotificationDot, StatusIconDanger } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';
import { categoryStyle } from '@/shared/model/payment-category-style';
import { circleIconRing } from '@/shared/ui/design';
import { categoryIconComponents } from '@/shared/ui/payment-category-icon-registry';
import { notificationHasWarningBadge } from '../model/event-badge';
import type { NotificationCategory, NotificationPaymentCategoryRef } from '../model/types';

/**
 * Иконка категории строки ленты (#744, Figma 651:6743 Category Icon
 * + 2329:149013) и страницы уведомления (#745, Figma 2333:184048): круг
 * с белым кольцом 2.5, внутри — снимок категории правила платежа (решение
 * владельца 14.10.2026: подложка цветом каталога #447, белый bold-глиф —
 * канон CategoryIcon карточек платежей, тикет #1348), либо фото объекта,
 * либо 3D-иллюстрация категории (PNG-эталоны из макета, канон §10 — не
 * генерируются), либо канонный BoldKey у платежей (в макете 2329:149014
 * платежи — белая bold-иконка на синем, 3D для категории нет); красная
 * точка непрочитанного — канон NotificationDot в левом-верхнем углу (как
 * у PropertyAvatar), только у строки.
 *
 * Приоритет кругляша: снимок категории правила (payment_category из
 * payload, платёжные события) → фото объекта из payload (#1275) →
 * 3D-иллюстрация/BoldKey категории. Снимка нет (строки до #1348) —
 * предыдущий канон: у платёжных фото объекта в ленте и BoldKey на странице.
 * Битое фото (фото/объект удалили после публикации) — откат по onError.
 *
 * Вариант row: круг 44, 3D 28 / bold-глиф 24 (пропорции макета строки),
 * Warning-бейдж 24 (Figma 2329:149016, Icon/Color/DangerWhite). Вариант
 * page: круг 96, 3D 61 / bold-глиф 52 (те же пропорции на круге 96),
 * бейдж 32, точка непрочитанного не рисуется — страница сама читает
 * уведомление. Белый кант бейджа красится через text-surface
 * (stroke="currentColor" в SVG — канон StatusIcon), отделяя бейдж от
 * тёмной 3D-иллюстрации.
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


/** Пропорции окружности варианта: круг, 3D-иллюстрация, BoldKey, бейдж
 * (3D 0.64 круга, bold 0.55 — как в макете строки #744). */
const ICON_SIZES = {
  row: { circle: 'h-11 w-11', image: 'h-7 w-7', boldKey: 'h-6 w-6', badge: 'h-6 w-6', badgeOffset: '-bottom-2 -right-2' },
  page: { circle: 'h-24 w-24', image: 'h-[61px] w-[61px]', boldKey: 'h-[52px] w-[52px]', badge: 'h-8 w-8', badgeOffset: '-bottom-2 -right-2' },
} as const;

export function NotificationCategoryIcon({
  category,
  eventType,
  unread,
  photoUrl,
  paymentCategory,
  variant = 'row',
  badge = true,
}: {
  readonly category: NotificationCategory;
  /** Тип события модели: бейдж ключится по нему (#1164) — «Оплата прошла»
   * и другие хорошие тарифные новости без бейджа. Кадр SSE его не несёт —
   * тост передаёт только badge=false. */
  readonly eventType?: string;
  readonly unread: boolean;
  /** Снимок категории правила платежа из payload (решение владельца
   * 14.10.2026, #1348): есть — иконка и цвет каталога #447 в круге вместо
   * фото объекта и 3D-иллюстрации; нет (строки до снимка) — предыдущий
   * канон. */
  readonly paymentCategory?: NotificationPaymentCategoryRef;
  /** Снимок фото объекта из payload (#1275): есть — фото в круге вместо
   * 3D-иллюстрации категории; битое (фото/объект удалили после публикации)
   * — откат к иллюстрации категории по onError. */
  readonly photoUrl?: string;
  /** row — строка ленты (#744), page — страница уведомления (#745). */
  readonly variant?: 'row' | 'page';
  /** Warning-бейдж («что-то не так», матрица event-badge): в ленте и на
   * странице рисуется, в тосте нового уведомления (#747) макет даёт
   * Badge=None — off. */
  readonly badge?: boolean;
}): JSX.Element {
  const visual = CATEGORY_VISUALS[category];
  const sizes = ICON_SIZES[variant];
  // Снимок категории правила — иконка и цвет каталога #447 (канон
  // CategoryIcon карточек платежей); у пользовательской категории слага
  // нет — style резолвится в дефолт пользовательских. Неизвестное имя
  // глифа молча падает туда же.
  const paymentStyle =
    paymentCategory === undefined
      ? undefined
      : categoryStyle(
          paymentCategory.source,
          paymentCategory.source === 'default' ? paymentCategory.slug : undefined,
        );
  const PaymentGlyph =
    paymentStyle === undefined ? undefined : (categoryIconComponents[paymentStyle.icon] ?? BoldOther);
  // Битое фото (404 стрима после удаления) — откат к 3D-иллюстрации
  // категории: строка ленты не остаётся с пустым кругом (#1275).
  const [photoBroken, setPhotoBroken] = useState(false);
  const showPhoto = PaymentGlyph === undefined && photoUrl !== undefined && !photoBroken;
  const circleBackground = showPhoto
    ? 'bg-surface-muted'
    : PaymentGlyph === undefined
      ? visual.className
      : undefined;
  // Размер живёт на внешнем wrapper: точка и бейдж якорятся к кругу, а сам
  // wrapper не растягивается флекс-родителем (страница #745 — колонка).
  return (
    <div className={cn('relative shrink-0', sizes.circle)} aria-hidden>
      <div
        className={cn(
          'flex h-full w-full items-center justify-center overflow-hidden rounded-pill',
          circleIconRing.white,
          circleBackground,
        )}
        style={paymentStyle !== undefined ? { backgroundColor: paymentStyle.color } : undefined}
      >
        {PaymentGlyph !== undefined && paymentStyle !== undefined ? (
          <PaymentGlyph className={cn(sizes.boldKey, 'text-white')} />
        ) : showPhoto ? (
          // <img>, не next/image: путь same-origin стриминга бэка
          // (ADR 0065), оптимизатору не передаётся — канон PropertyAvatar.
          <img
            src={photoUrl}
            alt=""
            className="h-full w-full object-cover"
            onError={() => setPhotoBroken(true)}
          />
        ) : visual.kind === 'image' ? (
          <Image src={visual.src} alt="" width={112} height={112} className={sizes.image} unoptimized />
        ) : (
          <BoldKey className={cn(sizes.boldKey, 'text-white')} />
        )}
      </div>
      {variant === 'row' && unread && <NotificationDot className="absolute left-0 top-0 h-3.5 w-3.5" />}
      {badge && eventType !== undefined && notificationHasWarningBadge(eventType) && (
        // Кант бейджа — stroke="currentColor" в самом SVG, красится под
        // поверхность ленты (прецедент features/payment-categories
        // category-icon): белая лента → text-surface.
        <StatusIconDanger
          className={cn('absolute', sizes.badgeOffset, sizes.badge, 'text-surface')}
          aria-hidden
        />
      )}
    </div>
  );
}
