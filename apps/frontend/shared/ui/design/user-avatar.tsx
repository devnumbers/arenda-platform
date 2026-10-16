'use client';

import { useState, type ComponentProps, type JSX } from 'react';
import { cn } from '@/shared/lib/cn';
import { BoldUser } from '@/shared/assets/icons';
import { CircleIcon } from './circle-icon';

export type UserAvatarProps = Omit<ComponentProps<'span'>, 'children'> & {
  /** Путь стриминга фото профиля (ADR 0065): null/undefined/'' — фото нет.
   * Битое фото (фото удалили, доступ отозван — 404 стрима) откатывается к
   * заглушке BoldUser (канон фолбэка #1275, решение #1286). */
  readonly photoUrl?: string | null;
  /** row — канон-круг 44 строк списков (глиф 24); hero — круг 96 шапок
   * экранов (глиф 52). */
  readonly size?: 'row' | 'hero';
  /** Подложка круга (CircleIcon): white — строки на белом, muted — на
   * серых карточках. */
  readonly variant?: 'white' | 'muted';
};

const SIZE_BOX: Record<NonNullable<UserAvatarProps['size']>, string> = {
  row: '',
  hero: 'h-24 w-24',
};

const SIZE_GLYPH: Record<NonNullable<UserAvatarProps['size']>, string> = {
  row: 'h-6 w-6',
  hero: 'h-[52px] w-[52px]',
};

/**
 * Круглый аватар человека — канон «фото профиля, иначе заглушка» (Figma
 * «Category Icon» 651:5921/699:8867; ADR 0065, решение #1286): собирает
 * CircleIcon + BoldUser, с фото круг клипает изображение. Поверхности:
 * строки участников/контактов/членов объекта, шапка приложения,
 * actor-кружки лент, шапки экранов (hero). Декоративен: имя рядом в
 * строке, у интерактивных обёрток доступное имя несёт кнопка/ссылка.
 */
export function UserAvatar({
  photoUrl,
  size = 'row',
  variant = 'white',
  className,
  ...props
}: UserAvatarProps): JSX.Element {
  // Битое фото живёт до размонтирования: строки списков кейуются по id,
  // повторная загрузка фото меняет ETag, не путь.
  const [photoBroken, setPhotoBroken] = useState(false);
  const showPhoto = photoUrl != null && photoUrl !== '' && !photoBroken;

  return (
    <CircleIcon
      variant={variant}
      aria-hidden
      className={cn(SIZE_BOX[size], showPhoto && 'overflow-hidden rounded-full', className)}
      {...props}
    >
      {showPhoto ? (
        <img
          src={photoUrl}
          alt=""
          className="h-full w-full object-cover"
          onError={() => setPhotoBroken(true)}
        />
      ) : (
        <BoldUser className={SIZE_GLYPH[size]} />
      )}
    </CircleIcon>
  );
}
