import type { JSX } from 'react';
import { BoldUser } from '@/shared/assets/icons';
import { CircleIcon } from '@/shared/ui/design';

/** Аватар-плейсхолдер 96px (Figma Category Icon 651:5921, Background=White):
 * серый круг с белым кольцом 2.5 и Bold/User 52 — канон CircleIcon с
 * размером 96 классом. Фото не загружается — аватар отложен (решение карты
 * #591: без кнопки «Добавить фото»). */
export function AvatarPlaceholder(): JSX.Element {
  return (
    <CircleIcon variant="white" aria-hidden className="h-24 w-24">
      <BoldUser className="h-[52px] w-[52px]" />
    </CircleIcon>
  );
}
