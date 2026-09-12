import type { JSX } from 'react';
import { BoldUser } from '@/shared/assets/icons';

/** Аватар-плейсхолдер 96px (Figma Category Icon 651:5921, Background=White):
 * серый круг с белым кольцом 2.5 и Bold/User 52. Фото не загружается —
 * аватар отложен (решение карты #591: без кнопки «Добавить фото»). */
export function AvatarPlaceholder(): JSX.Element {
  return (
    <span
      aria-hidden
      className="flex h-24 w-24 items-center justify-center rounded-pill bg-surface-muted shadow-[0_0_0_2.5px_var(--dl-surface)]"
    >
      <BoldUser className="h-[52px] w-[52px]" />
    </span>
  );
}
