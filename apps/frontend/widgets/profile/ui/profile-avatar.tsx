import type { JSX } from 'react';
import { cn } from '@/shared/lib/cn';
import { BoldUser } from '@/shared/assets/icons';
import { CircleIcon } from '@/shared/ui/design';

/**
 * Аватар профиля 96px (Figma Category Icon 651:5921, Background=White):
 * серый круг с белым кольцом 2.5 и Bold/User 52 — канон CircleIcon с
 * размером 96 классом; с фото кантом клипается изображение
 * (`overflow-hidden rounded-full`, канон CircleIcon). Плейсхолдер без
 * фото — прежний AvatarPlaceholder (аватар «отложен» был решением карты
 * #591; возвращён тикетом #1230). Потребитель передаёт уже готовый URL
 * показа (бастер/stage — mePhotoDisplay). Декоративен: имя рядом, у
 * управляемого круга на экране аккаунта aria-label несёт кнопка-обёртка.
 */
export function ProfileAvatar({ photoUrl }: { readonly photoUrl: string | null }): JSX.Element {
  return (
    <CircleIcon
      variant="white"
      aria-hidden
      className={cn('h-24 w-24', photoUrl !== null && 'overflow-hidden rounded-full')}
    >
      {photoUrl !== null ? (
        <img src={photoUrl} alt="" className="h-full w-full object-cover" />
      ) : (
        <BoldUser className="h-[52px] w-[52px]" />
      )}
    </CircleIcon>
  );
}
