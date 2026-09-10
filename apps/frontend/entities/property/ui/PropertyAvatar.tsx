import type { JSX } from 'react';
import { cn } from '@/shared/lib/cn';
import { BoldHome, NotificationDot } from '@/shared/assets/icons';

/**
 * Круглый аватар объекта (Figma Category Icon 651:5923, тикет #586): фото
 * объекта или дом-плейсхолдер #D3D7D9. Поверхность card — на серой карточке
 * списка «Объектов» (белый круг 64, кант серого — 1603:88779) с красной
 * точкой занятости/просрочки (Notification Dot 651:6759, запечённый кант);
 * row — на белой странице поиска (серый круг 44, кант белого — паттерн
 * SelectAvatar 888:19370). Точка — вне клипающего фото контейнера: круг
 * с overflow-hidden срезал бы её углы.
 */
export type PropertyAvatarProps = {
  readonly photoUrl?: string | null;
  /** card — 64 на серой карточке; row — 44 на белой странице. */
  readonly surface?: 'card' | 'row';
  /** Красная точка (просроченные операции ИЛИ «подошла к концу», #584). */
  readonly withAttentionDot?: boolean;
  readonly className?: string;
};

export function PropertyAvatar({
  photoUrl,
  surface = 'card',
  withAttentionDot = false,
  className,
}: PropertyAvatarProps): JSX.Element {
  const isCard = surface === 'card';
  return (
    <span className={cn('relative shrink-0', isCard ? 'h-16 w-16' : 'h-11 w-11', className)}>
      <span
        className={cn(
          'flex h-full w-full items-center justify-center overflow-hidden rounded-full',
          isCard ? 'bg-surface shadow-[0_0_0_2.5px_var(--dl-surface-muted)]' : 'bg-surface-muted shadow-[0_0_0_2.5px_var(--dl-surface)]',
        )}
      >
        {photoUrl ? (
          <img src={photoUrl} alt="" className="h-full w-full object-cover" />
        ) : (
          <BoldHome className={cn('text-[#D3D7D9]', isCard ? 'h-9 w-9' : 'h-6 w-6')} aria-hidden />
        )}
      </span>
      {withAttentionDot && (
        <NotificationDot className="absolute left-0 top-0 h-3.5 w-3.5" aria-hidden />
      )}
    </span>
  );
}
