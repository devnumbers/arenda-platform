import type { JSX } from 'react';
import { cn } from '@/shared/lib/cn';
import { BoldHome, NotificationDot } from '@/shared/assets/icons';

/**
 * Круглый аватар объекта (Figma Category Icon 651:5923, тикет #586): фото
 * объекта или дом-плейсхолдер #D3D7D9 — единственное место «лица объекта»,
 * размеры и канты всех поверхностей держит карта ниже. Поверхности:
 * card — на серой карточке списка «Объектов» (белый круг 64, кант серого —
 * 1603:88779) с красной точкой занятости/просрочки (Notification Dot
 * 651:6759, запечённый кант); row — на белой странице (серый круг 44,
 * кант белого — паттерн SelectAvatar 888:19370; поиск #601); hero — 96 в
 * шапках детали и правки (1186:44997, 1550:95852 — серый круг без канта);
 * feed — 24 в шапках групп ленты «Истории» (макет 2157-56876, #709 —
 * серый круг без канта, как hero); filter — 32 в строках «Объектов» шита
 * фильтров истории (макет 2050-158280, #711 — серый круг без канта, как
 * feed). Точка — вне клипающего фото контейнера: круг с overflow-hidden
 * срезал бы её углы.
 */
export type PropertyAvatarProps = {
  readonly photoUrl?: string | null;
  /** card — 64 на серой карточке; row — 44 на белой странице; hero — 96 в шапках; feed — 24 в шапках групп лент; filter — 32 в шите фильтров истории. */
  readonly surface?: 'card' | 'row' | 'hero' | 'feed' | 'filter';
  /** Красная точка (просроченные операции ИЛИ «подошла к концу», #584). */
  readonly withAttentionDot?: boolean;
  readonly className?: string;
};

const SURFACE_BOX: Record<NonNullable<PropertyAvatarProps['surface']>, string> = {
  card: 'h-16 w-16',
  row: 'h-11 w-11',
  hero: 'h-24 w-24',
  feed: 'h-6 w-6',
  filter: 'h-8 w-8',
};

const SURFACE_CIRCLE: Record<NonNullable<PropertyAvatarProps['surface']>, string> = {
  card: 'bg-surface shadow-[0_0_0_2.5px_var(--dl-surface-muted)]',
  row: 'bg-surface-muted shadow-[0_0_0_2.5px_var(--dl-surface)]',
  hero: 'bg-surface-muted',
  feed: 'bg-surface-muted',
  filter: 'bg-surface-muted',
};

const SURFACE_ICON: Record<NonNullable<PropertyAvatarProps['surface']>, string> = {
  card: 'h-9 w-9',
  row: 'h-6 w-6',
  hero: 'h-10 w-10',
  feed: 'h-3.5 w-3.5',
  filter: 'h-[18px] w-[18px]',
};

export function PropertyAvatar({
  photoUrl,
  surface = 'card',
  withAttentionDot = false,
  className,
}: PropertyAvatarProps): JSX.Element {
  return (
    <span className={cn('relative shrink-0', SURFACE_BOX[surface], className)}>
      <span
        className={cn(
          'flex h-full w-full items-center justify-center overflow-hidden rounded-full',
          SURFACE_CIRCLE[surface],
        )}
      >
        {photoUrl ? (
          <img src={photoUrl} alt="" className="h-full w-full object-cover" />
        ) : (
          <BoldHome className={cn('text-[#D3D7D9]', SURFACE_ICON[surface])} aria-hidden />
        )}
      </span>
      {withAttentionDot && (
        <NotificationDot className="absolute left-0 top-0 h-3.5 w-3.5" aria-hidden />
      )}
    </span>
  );
}
