'use client';

import { type ComponentProps, type JSX } from 'react';
import { cn } from '@/shared/lib/cn';
import { BoldUser } from '@/shared/assets/icons';
import { CirclePhoto } from './circle-photo';
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
  // Битое фото живёт до размонтирования (внутри CirclePhoto): строки
  // списков кейуются по id, повторная загрузка фото меняет ETag, не путь.
  // Клип включается по факту фото; после onError глиф остаётся в том же
  // круге — класс на отображение не влияет.
  const showPhoto = photoUrl != null && photoUrl !== '';

  return (
    <CircleIcon
      variant={variant}
      aria-hidden
      className={cn(SIZE_BOX[size], showPhoto && 'overflow-hidden rounded-full', className)}
      {...props}
    >
      <CirclePhoto
        photoUrl={photoUrl}
        glyph={<BoldUser className={SIZE_GLYPH[size]} />}
      />
    </CircleIcon>
  );
}
