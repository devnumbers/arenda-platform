'use client';

import { useState, type ComponentProps, type JSX } from 'react';
import { cn } from '@/shared/lib/cn';
import { BoldHome, NotificationDot } from '@/shared/assets/icons';
import { circleIconPair } from '@/shared/ui/design';

import { propertyTypeIcons } from '../lib/property-type-icons';
import type { PropertyType } from '../model/types';

/**
 * Круглый аватар объекта (Figma Category Icon 651:5923, тикет #586): фото
 * объекта или глиф-плейсхолдер по типу — канон «лица объекта» на
 * поверхностях этого компонента (карта поверхностей ниже). Вне его канона
 * «лицо объекта» рисуют ещё два потребителя снимков уведомлений (#1275):
 * NotificationCategoryIcon (фото в строке ленты) и PropertyCardAvatar на
 * странице уведомления — у обоих фолбэк битого фото отличается (иконка
 * категории / глиф типа), поэтому канон не переиспользован.
 * Поверхности: card — на серой карточке списка «Объектов» (белый круг 64,
 * кант серого — 1603:88779) с красной точкой занятости/просрочки
 * (Notification Dot 651:6759, запечённый кант); row — на белой странице
 * (серый круг 44, кант белого — паттерн SelectAvatar 888:19370; поиск
 * #601); hero — 96 в шапках детали и правки (1186:44997, 1550:95852 —
 * серый круг без канта); feed — 24 в шапках групп ленты «Истории» (макет
 * 2157-56876, #709 — серый круг без канта, как hero); filter — 32 в
 * строках «Объектов» шита фильтров истории (макет 2184-94261, #840 —
 * белый круг без канта, как у аватара участника). Точка — вне клипающего
 * фото контейнера: круг с overflow-hidden срезал бы её углы. Плейсхолдер
 * всех поверхностей: светлый круг + серый глиф типа #D3D7D9 (варианты
 * Category Icon 3099:151373…151403, карта #1217; реестр —
 * lib/property-type-icons; без типа — дом BoldHome).
 *
 * Корень — `flex`, не inline: span вне flex/block-родителя (медиа-блок
 * детали, карта #984) иначе схлопывается до размера глифа — размеры на
 * inline не действуют, круг жил размером дома (40 вместо 96). Глиф hero —
 * 52 = inset 22.73% макета (Figma Category Icon 2973:52664, круг 96;
 * карта #984).
 */
export type PropertyAvatarProps = ComponentProps<'span'> & {
  readonly photoUrl?: string | null;
  /** card — 64 на серой карточке; row — 44 на белой странице; hero — 96 в шапках; feed — 24 в шапках групп лент; filter — 32 в шите фильтров истории. */
  readonly surface?: 'card' | 'row' | 'hero' | 'feed' | 'filter';
  /** Тип объекта — выбирает глиф-плейсхолдер (Category Icon 3099:151373…). Без типа — дом BoldHome. */
  readonly type?: PropertyType;
  /** Красная точка (просроченные операции ИЛИ «подошла к концу», #584). */
  readonly withAttentionDot?: boolean;
};

const SURFACE_BOX: Record<NonNullable<PropertyAvatarProps['surface']>, string> = {
  card: 'h-16 w-16',
  row: 'h-11 w-11',
  hero: 'h-24 w-24',
  feed: 'h-6 w-6',
  filter: 'h-8 w-8',
};

const SURFACE_CIRCLE: Record<NonNullable<PropertyAvatarProps['surface']>, string> = {
  // Кантовые поверхности — канон «круг 44 с кольцом 2.5px» (circleIconPair):
  // card — белый круг на серой карточке, row — серый круг на белой странице.
  card: circleIconPair.muted,
  row: circleIconPair.white,
  hero: 'bg-surface-muted',
  feed: 'bg-surface-muted',
  filter: 'bg-surface',
};

const SURFACE_ICON: Record<NonNullable<PropertyAvatarProps['surface']>, string> = {
  card: 'h-9 w-9',
  row: 'h-6 w-6',
  hero: 'h-13 w-13',
  feed: 'h-3.5 w-3.5',
  filter: 'h-[18px] w-[18px]',
};

export function PropertyAvatar({
  photoUrl,
  surface = 'card',
  type,
  withAttentionDot = false,
  className,
  ...props
}: PropertyAvatarProps): JSX.Element {
  // Выборка из статичного реестра, не вызов: react-hooks/static-components.
  const Glyph = type !== undefined ? propertyTypeIcons[type] : BoldHome;
  // Битое фото (объект или фото удалили после рендера строки — 404 стрима)
  // откатывается к глифу типа: канон фолбэка #1275, поднятый из
  // notification-компонентов (решение #1286). Флаг живёт до размонтирования
  // или смены photoUrl (списки кейуются по id — перемонтирование).
  const [photoBroken, setPhotoBroken] = useState(false);

  return (
    <span
      className={cn('relative flex shrink-0', SURFACE_BOX[surface], className)}
      {...props}
    >
      <span
        className={cn(
          'flex h-full w-full items-center justify-center overflow-hidden rounded-full',
          SURFACE_CIRCLE[surface],
        )}
      >
        {photoUrl && !photoBroken ? (
          <img
            src={photoUrl}
            alt=""
            className="h-full w-full object-cover"
            onError={() => setPhotoBroken(true)}
          />
        ) : (
          <Glyph className={cn('text-[#D3D7D9]', SURFACE_ICON[surface])} aria-hidden />
        )}
      </span>
      {withAttentionDot && (
        <NotificationDot className="absolute left-0 top-0 h-3.5 w-3.5" aria-hidden />
      )}
    </span>
  );
}
